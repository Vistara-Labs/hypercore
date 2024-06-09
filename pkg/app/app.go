package app

import (
    "context"
    "fmt"
	"errors"
    "vistara-node/pkg/ports"
	"vistara-node/pkg/api/events"
	"vistara-node/pkg/defaults"
    "vistara-node/pkg/log"
    "vistara-node/pkg/models"
)

const (
	MetadataInterfaceName = "eth0"
)

type Config struct {
	RootStateDir    string
	MaximumRetry    int
	DefaultProvider string
}

type App struct {
	cfg   *Config
	ports *ports.Collection
}

func New(cfg *Config, ports *ports.Collection) App {
	return App{
		cfg:   cfg,
		ports: ports,
	}
}

// Create implements App. commands.go CreateMicroVM
func (a *App) Create(ctx context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
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

// Delete implements App.
func (a *App) Delete(ctx context.Context, vmid models.VMID) error {
    logger := log.GetLogger(ctx).WithField("action", "delete")

    logger.Debugf("Deleting microvm %v", vmid)

    vm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
        Name: vmid.Name(),
        Namespace: vmid.Namespace(),
        UID: vmid.UID(),
    })
    if err != nil {
        return fmt.Errorf("Getting MicroVM specs: %w", err)
    }

    logger.Infof("hypervisor provider is %v", vm.Spec.Provider)

    if err := a.ports.MicrovmProviders[vm.Spec.Provider].Stop(ctx, vm); err != nil {
        return fmt.Errorf("deleting microvm: %w", err)
    }

    // TODO delete from containerd

	return nil
}

func (a *App) Reconcile(ctx context.Context, vmid models.VMID) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile")

	logger.Debugf("Creating microvm in reconcile %v\n", vmid)

	vm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      vmid.Name(),
		Namespace: vmid.Namespace(),
		UID:       vmid.UID(),
	})

	if err != nil {
		return fmt.Errorf("Getting MicroVM spec to start VM: %w", err)
	}

	logger.Infof("hypervisor provider is %v", vm.Spec.Provider)

	// call chosen hypervisor to start the vm
	if err := a.ports.MicrovmProviders[vm.Spec.Provider].Start(ctx, vm); err != nil {
		return fmt.Errorf("starting microvm: %w", err)
	}

	return nil
}
