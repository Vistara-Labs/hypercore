package ports

import (
	"context"
	"vistara-node/pkg/models"
)

// MicroService is the port definition for a microvm service.
type MicroVMService interface {
	Start(ctx context.Context, vm *models.MicroVM, completionFn func(error)) error
	Stop(ctx context.Context, vm *models.MicroVM) error
	Pid(ctx context.Context, vm *models.MicroVM) (int, error)
	VSockPath(vm *models.MicroVM) string
}

// NetworkService is a port for a service that interacts with the network
// stack on the host machine.
type NetworkService interface {
	// IfaceCreate will create the network interface.
	IfaceCreate(ctx context.Context, input IfaceCreateInput) (*IfaceDetails, error)
	// IfaceDelete is used to delete a network interface
	IfaceDelete(ctx context.Context, input DeleteIfaceInput) error
	// IfaceExists will check if an interface with the given name exists
	IfaceExists(ctx context.Context, name string) (bool, error)
}

type IfaceCreateInput struct {
	// DeviceName is the name of the network interface to create on the host.
	DeviceName string
	// BridgeName is the name of the bridge to attach to. Only if this is a tap device and attach is true.
	BridgeName string
	// IP to bind the interface to
	IP4 string
}

type IfaceDetails struct {
	// DeviceName is the name of the network interface created on the host.
	DeviceName string
	// MAC is the MAC address of the created interface.
	MAC string
	// Index is the network interface index on the host.
	Index int
}

type DeleteIfaceInput struct {
	// DeviceName is the name of the network interface to delete from the host.
	DeviceName string
}
