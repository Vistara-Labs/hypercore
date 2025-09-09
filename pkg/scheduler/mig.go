package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"vistara/hypercore/pkg/types"
)

// MIGScheduler manages GPU allocations and scheduling
type MIGScheduler struct {
	mu          sync.RWMutex
	devices     map[string]*types.GPUDeviceInfo
	allocations map[string]*types.AllocationInfo
}

// NewMIGScheduler creates a new MIG scheduler
func NewMIGScheduler() (*MIGScheduler, error) {
	// Initialize NVML
	if err := nvml.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize NVML: %v", err)
	}

	scheduler := &MIGScheduler{
		devices:     make(map[string]*types.GPUDeviceInfo),
		allocations: make(map[string]*types.AllocationInfo),
	}

	// Initialize devices
	if err := scheduler.initializeDevices(); err != nil {
		return nil, err
	}

	return scheduler, nil
}

// initializeDevices discovers and initializes GPU devices
func (s *MIGScheduler) initializeDevices() error {
	count, err := nvml.DeviceGetCount()
	if err != nil {
		return fmt.Errorf("failed to get device count: %v", err)
	}

	for i := 0; i < count; i++ {
		device, err := nvml.DeviceGetHandleByIndex(i)
		if err != nil {
			return fmt.Errorf("failed to get device handle: %v", err)
		}

		name, err := device.GetName()
		if err != nil {
			return fmt.Errorf("failed to get device name: %v", err)
		}

		memory, err := device.GetMemoryInfo()
		if err != nil {
			return fmt.Errorf("failed to get memory info: %v", err)
		}

		migMode, err := device.GetMigMode()
		if err != nil {
			return fmt.Errorf("failed to get MIG mode: %v", err)
		}

		deviceInfo := &types.GPUDeviceInfo{
			ID:              fmt.Sprintf("gpu-%d", i),
			Name:            name,
			TotalMemory:     int64(memory.Total),
			AvailableMemory: int64(memory.Free),
			MIGEnabled:      migMode.Current == nvml.DEVICE_MIG_ENABLE,
			Status:          "available",
			Allocations:     make([]string, 0),
		}

		s.mu.Lock()
		s.devices[deviceInfo.ID] = deviceInfo
		s.mu.Unlock()
	}

	return nil
}

// AllocateGPU allocates GPU resources for a workload
func (s *MIGScheduler) AllocateGPU(workloadID string, profile types.MIGProfile) (*types.AllocationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if workload already has an allocation
	if _, exists := s.allocations[workloadID]; exists {
		return nil, fmt.Errorf("workload %s already has an allocation", workloadID)
	}

	// Find suitable device
	var selectedDevice *types.GPUDeviceInfo
	for _, device := range s.devices {
		if device.AvailableMemory >= int64(profile.MemoryGB*1024*1024*1024) {
			selectedDevice = device
			break
		}
	}

	if selectedDevice == nil {
		return nil, fmt.Errorf("no suitable device found for the requested profile")
	}

	// Create allocation
	allocation := &types.AllocationInfo{
		WorkloadID:  workloadID,
		DeviceID:    selectedDevice.ID,
		Profile:     profile,
		AllocatedAt: time.Now().Unix(),
		Status:      "allocated",
	}

	// Update device info
	selectedDevice.AvailableMemory -= int64(profile.MemoryGB * 1024 * 1024 * 1024)
	selectedDevice.Allocations = append(selectedDevice.Allocations, workloadID)

	// Store allocation
	s.allocations[workloadID] = allocation
	return allocation, nil
}

// DeallocateGPU deallocates GPU resources for a workload
func (s *MIGScheduler) DeallocateGPU(workloadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allocation, exists := s.allocations[workloadID]
	if !exists {
		return fmt.Errorf("no allocation found for workload %s", workloadID)
	}

	device := s.devices[allocation.DeviceID]
	if device != nil {
		device.AvailableMemory += int64(allocation.Profile.MemoryGB * 1024 * 1024 * 1024)
		// Remove workload from allocations
		for i, wID := range device.Allocations {
			if wID == workloadID {
				device.Allocations = append(device.Allocations[:i], device.Allocations[i+1:]...)
				break
			}
		}
	}

	delete(s.allocations, workloadID)
	return nil
}

// GetWorkloadAllocation returns allocation information for a workload
func (s *MIGScheduler) GetWorkloadAllocation(workloadID string) (*types.AllocationInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allocation, exists := s.allocations[workloadID]
	if !exists {
		return nil, fmt.Errorf("no allocation found for workload %s", workloadID)
	}

	return allocation, nil
}

// GetAllAllocations returns all current allocations
func (s *MIGScheduler) GetAllAllocations() map[string]*types.AllocationInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.allocations
}

// GetAvailableDevices returns all available GPU devices
func (s *MIGScheduler) GetAvailableDevices() ([]types.GPUDeviceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]types.GPUDeviceInfo, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, *device)
	}

	return devices, nil
}

// GetDeviceUtilization returns device utilization information
func (s *MIGScheduler) GetDeviceUtilization() (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	utilization := make(map[string]float64)
	for id, device := range s.devices {
		utilizationPercent := float64(device.TotalMemory-device.AvailableMemory) / float64(device.TotalMemory) * 100
		utilization[id] = utilizationPercent
	}

	return utilization, nil
}

// CleanupExpiredAllocations removes expired allocations
func (s *MIGScheduler) CleanupExpiredAllocations(timeoutSeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	for workloadID, allocation := range s.allocations {
		if allocation.ExpiresAt > 0 && allocation.ExpiresAt < now {
			device := s.devices[allocation.DeviceID]
			if device != nil {
				device.AvailableMemory += int64(allocation.Profile.MemoryGB * 1024 * 1024 * 1024)
				// Remove workload from allocations
				for i, wID := range device.Allocations {
					if wID == workloadID {
						device.Allocations = append(device.Allocations[:i], device.Allocations[i+1:]...)
						break
					}
				}
			}
			delete(s.allocations, workloadID)
		}
	}

	return nil
}
