package models

// This state represents the state of the entire Hypercore MicroVM.
// The state for the Firecracker MVM itself is represented in ports.MicroVMState.
type MicroVMState string

const (
	PendingState  = "pending"
	CreatedState  = "created"
	FailedState   = "failed"
	DeletingState = "deleting"
)

type VmMetadata struct {
	Provider  string `json:"provider"`
	VCPU      int32  `json:"vcpu" validate:"required,gte=1"`
	Memory    int32  `json:"memory" validate:"required,gte=128"`
	HostIface string `json:"host_iface"`
}

// MicroVM represents a microvm machine that is created via a provider.
type MicroVM struct {
	// ID is the identifier for the microvm.
	ID VMID `json:"id"`
	// Version is the version for the microvm definition.
	Version int `json:"version"`
	// Spec is the specification of the microvm.
	Spec MicroVMSpec `json:"spec"`
	// Status is the runtime status of the microvm.
	Status MicroVMStatus `json:"status"`
}

// MicroVMSpec represents the specification of a microvm machine.
type MicroVMSpec struct {
	// Provider specifies the name of the microvm provider to use.
	Provider string `json:"provider"`
	// Kernel specifies the kernel and its argments to use.
	Kernel string `json:"kernel" validate:"omitempty"`
	// VCPU specifies how many vcpu the machine will be allocated.
	VCPU int32 `json:"vcpu" validate:"required,gte=1,lte=64"`
	// MemoryInMb is the amount of memory in megabytes that the machine will be allocated.
	MemoryInMb int32 `json:"memory_inmb" validate:"required,gte=1024,lte=32768"`
	// HostNetDev is the device to use for passing traffic through the TAP device
	HostNetDev string `json:"host_net_dev" validate:"omitempty"`
	RootfsPath string `json:"rootfs_path" validate:"omitempty"`
	ImagePath  string `json:"image_path" validate:"omitempty"`
	GuestMAC   string `json:"guest_mac" validate:"omitempty"`
	ImageRef   string `json:"image_ref" validate:"omitempty"`
	// CreatedAt indicates the time the microvm was created at.
	CreatedAt int64 `json:"created_at" validate:"omitempty,datetimeInPast"`
	// UpdatedAt indicates the time the microvm was last updated.
	UpdatedAt int64 `json:"updated_at" validate:"omitempty,datetimeInPast"`
	// DeletedAt indicates the time the microvm was marked as deleted.
	DeletedAt int64 `json:"deleted_at" validate:"omitempty,datetimeInPast"`
}

// MicroVMStatus contains the runtime status of the microvm.
type MicroVMStatus struct {
	// State stores information about the last known state of the vm and the spec.
	State MicroVMState `json:"state"`
	// Retry is a counter about how many times we retried to reconcile.
	Retry int `json:"retry"`
	// NotBefore tells the system to do not reconcile until given timestamp.
	NotBefore int64 `json:"not_before" validate:"omitempty"`
}

// ContainerImage represents the address of a OCI image.
type ContainerImage string
