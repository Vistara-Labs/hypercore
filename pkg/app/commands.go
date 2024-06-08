package app

import (
	"context"
	"fmt"

	// "vistara-node/pkg/api/events"
	"errors"
	"vistara-node/pkg/api/events"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"
)

const (
	MetadataInterfaceName = "eth0"
)

// Create implements App. commands.go CreateMicroVM
func (a *app) Create(ctx context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("vm", "app")

	if vm.ID.IsEmpty() {
		return nil, errors.New("empty vmID")
	}

	if vm.Spec.Provider == "" {
		vm.Spec.Provider = a.cfg.DefaultProvider
	}

	logger.Infof("Hypervisor provider: %s", vm.Spec.Provider)

	// vm.Spec.CreatedAt = a.ports.Clock().Unix()
	vm.Status.State = models.PendingState
	vm.Status.Retry = 0

	// Saves the microvm spec to containerd and get the microvm
	createdVm, err := a.ports.Repo.Save(ctx, vm)
	if err != nil {
		return nil, fmt.Errorf("saving microvm: %w", err)
	}

	// Publish a MicroVMCreated event to a queue.
	if err := a.ports.EventService.Publish(
		ctx,
		defaults.TopicMicroVMEvents,
		&events.MicroVMSpecCreated{
			ID:        vm.ID.Name(),
			Namespace: vm.ID.Namespace(),
			UID:       vm.ID.UID(),
		}); err != nil {
		return nil, fmt.Errorf("publishing microVM created event, vmid %s: %w", vm.ID.Name(), err)
	}

	return createdVm, nil
}

// Start implements App.
func (a *app) Start(ctx context.Context, vm *models.MicroVM) error {
	return fmt.Errorf("error starting, not implemented")
}

// Delete implements App.
func (*app) Delete(ctx context.Context, id string) error {
	panic("unimplemented")
}

// Metrics implements App.
func (*app) Metrics(ctx context.Context, id models.VMID) (ports.MachineMetrics, error) {
	panic("unimplemented")
}

// State implements App.
func (*app) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	panic("unimplemented")
}
