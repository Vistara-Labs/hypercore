package processors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"vistara-node/pkg/api/events"
	"vistara-node/pkg/app"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"
	"vistara-node/pkg/queue"

	"github.com/sirupsen/logrus"
)

type VMProcessor struct {
	app        app.App
	commandSvc ports.MicroVMService
	eventSvc   ports.EventService
	queue      queue.Queue
}

// this is the constructor for the VMProcessor, called from initProcessors,
// similar to initControllers in flintlockd
func NewVMProcessor(commandSvc ports.MicroVMService, eventSvc ports.EventService) *VMProcessor {
	return &VMProcessor{
		commandSvc: commandSvc,
		eventSvc:   eventSvc,
		queue:      queue.NewSimpleSyncQueue(),
	}
}

func (p *VMProcessor) Run(ctx context.Context, numWorkers int) error {
	logger := log.GetLogger(ctx).WithField("processor", "vm")
	ctx = log.WithLogger(ctx, logger)

	logger.Debug("Starting VM processor with %d workers", numWorkers)

	go func() {
		<-ctx.Done()
		p.queue.Shutdown()
	}()

	wg := &sync.WaitGroup{}
	logger.Info("Starting event listener")
	wg.Add(1)

	go func() {
		defer wg.Done()
		p.runEventListener(ctx)
	}()

	logger.Info("Starting workers", "num_workers", numWorkers)
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()

			for p.processQueue(ctx) {
			}
		}()
	}
	<-ctx.Done()
	logger.Info("waiting for workers to finish")
	wg.Wait()
	logger.Info("workers finished")

	return nil
}

func (p *VMProcessor) runEventListener(ctx context.Context) {
	logger := log.GetLogger(ctx)
	evtCh, errCh := p.eventSvc.SubscribeTopic(ctx, defaults.TopicMicroVMEvents)
	logger.Info("subscribed defaults.TopicMicroVMEvents : %v ", defaults.TopicMicroVMEvents)

	for {
		select {
		case <-ctx.Done():
			if cerr := ctx.Err(); cerr != nil && !errors.Is(cerr, context.Canceled) {
				logger.Errorf("canceling event loop: %s", cerr)
			}
			return
		case evt := <-evtCh:
			logger.Infof("received event %T", evt)
			if err := p.handleEvent(evt, logger); err != nil {
				logger.Errorf("resyncing specs: %s", err)
			}
		case evtErr := <-errCh:
			logger.Errorf("error from event service %s", evtErr)
			return
		}
	}

}

func (p *VMProcessor) handleEvent(envelope *ports.EventEnvelope, logger *logrus.Entry) error {
	var name, namespace, uid string

	logger.Infof("received event %T", envelope.Event)

	switch eventType := envelope.Event.(type) {
	case *events.MicroVMSpecCreated:
		created, _ := envelope.Event.(*events.MicroVMSpecCreated)
		name = created.ID
		namespace = created.Namespace
		uid = created.UID
	case *events.MicroVMSpecDeleted:
		// Deleted vmspec doesn't need to go into the queue
		// implement a better way to handle this
		return nil
	case *events.MicroVMSpecUpdated:
		updated, _ := envelope.Event.(*events.MicroVMSpecUpdated)
		name = updated.ID
		namespace = updated.Namespace
		uid = updated.UID
	default:
		logger.Debugf("unhandled event type (%T) received", eventType)
		return nil
	}

	vmid, err := models.NewVMID(name, namespace, uid)
	if err != nil {
		return fmt.Errorf("getting vmid from event data: %w", err)
	}

	logger.Debugf("enqueing vmid %s", vmid)
	p.queue.Enqueue(vmid.String())

	return nil
}

func (p *VMProcessor) processQueue(ctx context.Context) bool {
	logger := log.GetLogger(ctx)
	item, shutdown := p.queue.Dequeue()
	logger.Infof("item in procecssqueue %s, %s", item, shutdown)
	if shutdown {
		return false
	}

	// Log when the function starts
	logger.Debug("Processing queue")

	id, ok := item.(string)

	logger.Debug(item)
	if !ok {
		logger.Error("invalid item type in queues")
		// exit the function if the item is not a string
		os.Exit(1)
		return true
	}

	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		logger.WithError(err).Error("error parsing vmid")
		return true
	}

	err = p.commandSvc.Reconcile(ctx, *vmid)

	if err != nil {
		logger.WithError(err).Error("error creating microvm")
	}

	return true
}
