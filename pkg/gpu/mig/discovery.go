package mig

import (
	"fmt"
	"log"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"vistara/hypercore/pkg/types"
)

// GPUDeviceInfo wraps NVML device with additional metadata
type GPUDeviceInfo struct {
	Handle nvml.Device
	UUID   string
	Index  int
	Name   string
	Memory int64
}

// DiscoverGPUs discovers all available GPU devices
func DiscoverGPUs() ([]GPUDeviceInfo, error) {
	// Initialize NVML
	if err := nvml.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize NVML: %v", err)
	}
	defer nvml.Shutdown()

	count, err := nvml.DeviceGetCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get device count: %v", err)
	}

	var devices []GPUDeviceInfo
	for i := 0; i < count; i++ {
		device, err := nvml.DeviceGetHandleByIndex(i)
		if err != nil {
			log.Printf("Failed to get device handle for index %d: %v", i, err)
			continue
		}

		uuid, err := device.GetUUID()
		if err != nil {
			log.Printf("Failed to get UUID for device %d: %v", i, err)
			continue
		}

		name, err := device.GetName()
		if err != nil {
			log.Printf("Failed to get name for device %d: %v", i, err)
			name = "Unknown"
		}

		memory, err := device.GetMemoryInfo()
		if err != nil {
			log.Printf("Failed to get memory info for device %d: %v", i, err)
			memory = &nvml.Memory{Total: 0}
		}

		devices = append(devices, GPUDeviceInfo{
			Handle: device,
			UUID:   uuid,
			Index:  i,
			Name:   name,
			Memory: int64(memory.Total),
		})
	}

	return devices, nil
}

// GetDeviceInfo returns detailed information about a specific device
func GetDeviceInfo(device nvml.Device) (*types.GPUDeviceInfo, error) {
	uuid, err := device.GetUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to get UUID: %v", err)
	}

	name, err := device.GetName()
	if err != nil {
		return nil, fmt.Errorf("failed to get name: %v", err)
	}

	memory, err := device.GetMemoryInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %v", err)
	}

	return &types.GPUDeviceInfo{
		Handle: device,
		UUID:   uuid,
		Name:   name,
		Memory: int64(memory.Total),
	}, nil
}