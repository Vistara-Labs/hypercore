package types

// MIGProfile represents a Multi-Instance GPU profile configuration
type MIGProfile struct {
	ID          string `json:"id"`
	MemoryGB    int    `json:"memory_gb"`
	ComputeUtil int    `json:"compute_util"`
}

// GPUDeviceInfo contains information about a GPU device
type GPUDeviceInfo struct {
	Handle interface{} `json:"-"` // NVML device handle
	UUID   string      `json:"uuid"`
	Index  int         `json:"index"`
	Name   string      `json:"name"`
	Memory int64       `json:"memory"` // Memory in bytes
}

// WorkloadRequest represents a request for GPU allocation
type WorkloadRequest struct {
	ID      string     `json:"id"`
	Profile MIGProfile `json:"profile"`
	Timeout int        `json:"timeout"` // Timeout in seconds
}

// AllocationInfo tracks GPU allocation state
type AllocationInfo struct {
	WorkloadID string     `json:"workload_id"`
	DeviceID   string     `json:"device_id"`
	Profile    MIGProfile `json:"profile"`
	Status     string     `json:"status"` // "allocated", "failed", "released"
	CreatedAt  int64      `json:"created_at"`
}