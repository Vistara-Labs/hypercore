package processors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"vistara-node/pkg/api/events"
	"vistara-node/pkg/app"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"github.com/sirupsen/logrus"
)

type VMProcessor struct {
	commandSvc app.App
	eventSvc   ports.EventService
}

// this is the constructor for the VMProcessor, called from initProcessors,
// similar to initControllers in flintlockd
func NewVMProcessor(commandSvc app.App, eventSvc ports.EventService) *VMProcessor {
	return &VMProcessor{
		commandSvc: commandSvc,
		eventSvc:   eventSvc,
	}
}

func (p *VMProcessor) Run(ctx context.Context) error {
	logger := log.GetLogger(ctx).WithField("processor", "vm")
	ctx = log.WithLogger(ctx, logger)

	go func() {
		<-ctx.Done()
	}()

	wg := &sync.WaitGroup{}
	logger.Info("Starting event listener")
	wg.Add(1)

	go func() {
		defer wg.Done()
		p.runEventListener(ctx)
	}()

	<-ctx.Done()
	wg.Wait()

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
	logger.Infof("received event %T", envelope.Event)

	switch eventType := envelope.Event.(type) {
	case *events.MicroVMSpecCreated:
		created := envelope.Event.(*events.MicroVMSpecCreated)

		vmid, err := models.NewVMID(created.ID, created.Namespace, created.UID)
		if err != nil {
			return fmt.Errorf("getting vmid from event data: %w", err)
		}

		if err = p.commandSvc.Reconcile(context.Background(), *vmid); err != nil {
			return fmt.Errorf("error creating microvm: %w", err)
		}
	default:
		logger.Debugf("unhandled event type (%T) received", eventType)
		return nil
	}

	return nil
}
