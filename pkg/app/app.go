package app

import (
	"context"
	"errors"
	"fmt"
	"vistara-node/pkg/api/types"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"
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

	logger.Infof("Hypervisor provider: %s", vm.Spec.Provider)

	vm.Status.State = models.PendingState
	vm.Status.Retry = 0

	// Saves the microvm spec to containerd and get the microvm
	createdVm, err := a.ports.Repo.Save(ctx, vm)
	if err != nil {
		return nil, fmt.Errorf("saving microvm: %w", err)
	}

	provider, ok := a.ports.MicrovmProviders[vm.Spec.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", vm.Spec.Provider)
	}

	// call chosen hypervisor to start the vm
	if err := provider.Start(ctx, vm); err != nil {
		return nil, fmt.Errorf("starting microvm: %w", err)
	}

	return createdVm, nil
}

func (a *App) GetAll(ctx context.Context) ([]*models.MicroVM, error) {
	return a.ports.Repo.GetAll(ctx)
}

func (a *App) GetRuntimeData(ctx context.Context, vm *models.MicroVM) (*types.MicroVMRuntimeData, error) {
	return a.ports.MicrovmProviders[vm.Spec.Provider].GetRuntimeData(ctx, vm)
}

// Delete implements App.
func (a *App) Delete(ctx context.Context, vmid models.VMID) error {
	logger := log.GetLogger(ctx).WithField("action", "delete")

	logger.Debugf("Deleting microvm %v", vmid)

	vm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      vmid.Name(),
		Namespace: vmid.Namespace(),
		UID:       vmid.UID(),
	})
	if err != nil {
		return fmt.Errorf("getting MicroVM specs: %w", err)
	}

	logger.Infof("hypervisor provider is %v", vm.Spec.Provider)

	if err := a.ports.MicrovmProviders[vm.Spec.Provider].Stop(ctx, vm); err != nil {
		return fmt.Errorf("deleting microvm: %w", err)
	}

	if err = a.ports.Repo.Delete(ctx, ports.RepositoryGetOptions{
		Name:      vmid.Name(),
		Namespace: vmid.Namespace(),
		UID:       vmid.UID(),
	}); err != nil {
		return fmt.Errorf("deleting from containerd: %w", err)
	}

	return nil
}

func (a *App) Reconcile(ctx context.Context, vmid models.VMID) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile")

	logger.Debugf("Creating microvm in reconcile %v\n", vmid)

	return nil
}
