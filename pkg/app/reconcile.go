package app

import (
	"context"
	"fmt"

	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"
)

func NewStartConfig(vm *models.MicroVM, vmSvc ports.MicroVMService) error {
	return &startConfig{
		vm:    vm,
		vmSvc: vmSvc,
	}
}

type startConfig struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
}

// Error implements error.
func (*startConfig) Error() string {
	panic("unimplemented")
}

// Create implements App. commands.go CreateMicroVM
func (a *app) Reconcile(ctx context.Context, vmid models.VMID) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile")

	logger.Infof("Creating microvm in reconcile %v\n", vmid)

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
