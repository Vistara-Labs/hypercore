package scheduler

import (
	"fmt"
	"sync"
	"time"

	"vistara/hypercore/pkg/gpu/mig"
	"vistara/hypercore/pkg/types"
)

// MIGScheduler manages GPU allocation using MIG
type MIGScheduler struct {
	manager     *mig.Manager
	allocations map[string]*types.AllocationInfo // workloadID -> allocation
	mu          sync.RWMutex
}

// NewMIGScheduler creates a new MIG scheduler
func NewMIGScheduler() (*MIGScheduler, error) {
	manager, err := mig.NewManager()
	if err != nil {
		return nil, err
	}

	return &MIGScheduler{
		manager:     manager,
		allocations: make(map[string]*types.AllocationInfo),
	}, nil
}

// AllocateGPU allocates a GPU with the specified MIG profile
func (s *MIGScheduler) AllocateGPU(workloadID string, profile types.MIGProfile) (*types.AllocationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if workload already has allocation
	if _, exists := s.allocations[workloadID]; exists {
		return nil, fmt.Errorf("workload %s already has GPU allocation", workloadID)
	}

	// Find available GPU
	devices, err := s.manager.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %v", err)
	}

	// Simple round-robin allocation
	for _, device := range devices {
		// Check if device is available (not allocated to another workload)
		available := true
		for _, allocation := range s.allocations {
			if allocation.DeviceID == device.UUID && allocation.Status == "allocated" {
				available = false
				break
			}
		}

		if !available {
			continue
		}

		// Create MIG device
		instance, err := s.manager.CreateMIGDevice(device.UUID, profile)
		if err != nil {
			log.Printf("Failed to create MIG device on %s: %v", device.UUID, err)
			continue // Try next device
		}

		// Create allocation record
		allocation := &types.AllocationInfo{
			WorkloadID: workloadID,
			DeviceID:   device.UUID,
			Profile:    profile,
			Status:     "allocated",
			CreatedAt:  time.Now().Unix(),
		}

		// Mark as allocated
		s.allocations[workloadID] = allocation

		log.Printf("Allocated GPU %s with profile %s to workload %s", device.UUID, profile.ID, workloadID)
		return allocation, nil
	}

	return nil, fmt.Errorf("no available GPU for workload %s", workloadID)
}

// DeallocateGPU deallocates a GPU from a workload
func (s *MIGScheduler) DeallocateGPU(workloadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allocation, exists := s.allocations[workloadID]
	if !exists {
		return fmt.Errorf("workload %s not found", workloadID)
	}

	// Destroy MIG instance
	if err := s.manager.DestroyMIGDevice(allocation.DeviceID + "-" + workloadID); err != nil {
		log.Printf("Failed to destroy MIG device: %v", err)
		// Continue with deallocation even if cleanup fails
	}

	// Update status
	allocation.Status = "released"

	// Remove from active allocations
	delete(s.allocations, workloadID)

	log.Printf("Deallocated GPU from workload %s", workloadID)
	return nil
}

// GetWorkloadAllocation returns the allocation info for a workload
func (s *MIGScheduler) GetWorkloadAllocation(workloadID string) (*types.AllocationInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allocation, exists := s.allocations[workloadID]
	if !exists {
		return nil, fmt.Errorf("workload %s not found", workloadID)
	}

	return allocation, nil
}

// GetAllAllocations returns all current allocations
func (s *MIGScheduler) GetAllAllocations() map[string]*types.AllocationInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*types.AllocationInfo)
	for workloadID, allocation := range s.allocations {
		result[workloadID] = allocation
	}

	return result
}

// GetAvailableDevices returns devices that are available for allocation
func (s *MIGScheduler) GetAvailableDevices() ([]mig.GPUDeviceInfo, error) {
	devices, err := s.manager.GetDevices()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var available []mig.GPUDeviceInfo
	for _, device := range devices {
		allocated := false
		for _, allocation := range s.allocations {
			if allocation.DeviceID == device.UUID && allocation.Status == "allocated" {
				allocated = true
				break
			}
		}

		if !allocated {
			available = append(available, device)
		}
	}

	return available, nil
}

// GetDeviceUtilization returns utilization statistics for all devices
func (s *MIGScheduler) GetDeviceUtilization() (map[string]float64, error) {
	devices, err := s.manager.GetDevices()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	utilization := make(map[string]float64)
	for _, device := range devices {
		allocatedCount := 0
		for _, allocation := range s.allocations {
			if allocation.DeviceID == device.UUID && allocation.Status == "allocated" {
				allocatedCount++
			}
		}

		// Calculate utilization based on MIG instances
		migInstances, err := s.manager.GetMIGInstances(device.UUID)
		if err != nil {
			utilization[device.UUID] = 0.0
			continue
		}

		if len(migInstances) > 0 {
			utilization[device.UUID] = float64(allocatedCount) / float64(len(migInstances)) * 100.0
		} else {
			utilization[device.UUID] = 0.0
		}
	}

	return utilization, nil
}

// CleanupExpiredAllocations removes allocations that have exceeded their timeout
func (s *MIGScheduler) CleanupExpiredAllocations(timeoutSeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	var expiredWorkloads []string

	for workloadID, allocation := range s.allocations {
		if now-allocation.CreatedAt > int64(timeoutSeconds) {
			expiredWorkloads = append(expiredWorkloads, workloadID)
		}
	}

	for _, workloadID := range expiredWorkloads {
		log.Printf("Cleaning up expired allocation for workload %s", workloadID)
		if err := s.DeallocateGPU(workloadID); err != nil {
			log.Printf("Failed to cleanup workload %s: %v", workloadID, err)
		}
	}

	return nil
}