package app

import (
	"context"
	"fmt"

	// "vistara-node/pkg/api/events"
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

	// TODO: add validation here.

	if vm.ID.IsEmpty() {
		name, err := a.ports.IdentifierService.GenerateRandom()
		if err != nil {
			return nil, fmt.Errorf("generating random name for microvm: %w", err)
		}

		vmid, err := models.NewVMID(name, defaults.MicroVMNamespace, "")
		if err != nil {
			return nil, fmt.Errorf("creating microvm vmid: %w", err)
		}
		vm.ID = *vmid
	}

	if vm.Spec.Provider == "" {
		vm.Spec.Provider = a.cfg.DefaultProvider
	}
	a.addMetadataInterface(vm)

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


func (a *app) addMetadataInterface(mvm *models.MicroVM) {
	for i := range mvm.Spec.NetworkInterfaces {
		netInt := mvm.Spec.NetworkInterfaces[i]
		if netInt.GuestDeviceName == MetadataInterfaceName {
			return
		}
	}

	interfaces := []models.NetworkInterface{
		{
			GuestDeviceName:       MetadataInterfaceName,
			Type:                  models.IfaceTypeTap,
			AllowMetadataRequests: true,
			GuestMAC:              "AA:FF:00:00:00:01",
			StaticAddress: &models.StaticAddress{
				Address: "169.254.0.1/16",
			},
		},
	}
	interfaces = append(interfaces, mvm.Spec.NetworkInterfaces...)
	mvm.Spec.NetworkInterfaces = interfaces

	return
}
