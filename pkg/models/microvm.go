package models

type MicroVM struct {
	ID   string      `json:"id"`
	Spec MicroVMSpec `json:"spec"`
}

type MicroVMSpec struct {
	Provider   string `json:"provider"`
	Kernel     string `json:"kernel"       validate:"omitempty"`
	VCPU       int32  `json:"vcpu"         validate:"required,gte=1,lte=64"`
	MemoryInMb int32  `json:"memory_inmb"  validate:"required,gte=1024,lte=32768"`
	HostNetDev string `json:"host_net_dev" validate:"omitempty"`
	RootfsPath string `json:"rootfs_path"  validate:"omitempty"`
	ImagePath  string `json:"image_path"   validate:"omitempty"`
	GuestMAC   string `json:"guest_mac"    validate:"omitempty"`
}
