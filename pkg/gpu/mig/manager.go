package mig

import (
	"fmt"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"vistara/hypercore/pkg/types"
)

var (
	// Default MIG profiles for common use cases
	defaultProfiles = map[string]types.MIGProfile{
		"1g.5gb":  {ID: "1g.5gb", MemoryGB: 5, ComputeUtil: 1},
		"2g.10gb": {ID: "2g.10gb", MemoryGB: 10, ComputeUtil: 2},
		"3g.20gb": {ID: "3g.20gb", MemoryGB: 20, ComputeUtil: 3},
		"4g.20gb": {ID: "4g.20gb", MemoryGB: 20, ComputeUtil: 4},
		"7g.40gb": {ID: "7g.40gb", MemoryGB: 40, ComputeUtil: 7},
	}

	// MIG instance tracking
	migInstances = make(map[string][]MIGInstance)
	migMutex     sync.RWMutex
)

// MIGInstance represents a created MIG instance
type MIGInstance struct {
	ID               string
	GPUInstance      nvml.GpuInstance
	ComputeInstance  nvml.ComputeInstance
	Profile          types.MIGProfile
	CreatedAt        time.Time
	WorkloadID       string
}

// Manager handles MIG operations
type Manager struct {
	devices []GPUDeviceInfo
	mu      sync.RWMutex
}

// NewManager creates a new MIG manager
func NewManager() (*Manager, error) {
	devices, err := DiscoverGPUs()
	if err != nil {
		return nil, err
	}
	return &Manager{devices: devices}, nil
}

// GetDevices returns all discovered GPU devices
func (m *Manager) GetDevices() ([]GPUDeviceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.devices, nil
}

// GetDefaultProfiles returns available MIG profiles
func (m *Manager) GetDefaultProfiles() map[string]types.MIGProfile {
	return defaultProfiles
}

// CreateMIGDevice creates a MIG device with the specified profile
func (m *Manager) CreateMIGDevice(deviceID string, profile types.MIGProfile) (*MIGInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, err := m.findDevice(deviceID)
	if err != nil {
		return nil, err
	}

	// Enable MIG mode if not enabled
	if err := EnableMIGMode(device.Handle); err != nil {
		return nil, fmt.Errorf("failed to enable MIG mode: %v", err)
	}

	// Get GPU instance size based on profile
	giSize := nvml.GI_SIZE_1GB
	switch profile.ComputeUtil {
	case 2:
		giSize = nvml.GI_SIZE_2GB
	case 3:
		giSize = nvml.GI_SIZE_3GB
	case 4:
		giSize = nvml.GI_SIZE_4GB
	case 7:
		giSize = nvml.GI_SIZE_7GB
	}

	// Create GPU instance
	gi, err := device.Handle.CreateGpuInstance(&giSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create GPU instance: %v", err)
	}

	// Create Compute Instance
	ciProfile := nvml.CI_PROFILE_1_SLICE
	ci, err := gi.CreateComputeInstance(&ciProfile)
	if err != nil {
		gi.Destroy() // Cleanup on failure
		return nil, fmt.Errorf("failed to create compute instance: %v", err)
	}

	instance := &MIGInstance{
		ID:              fmt.Sprintf("%s-%d", deviceID, time.Now().UnixNano()),
		GPUInstance:     gi,
		ComputeInstance: ci,
		Profile:         profile,
		CreatedAt:       time.Now(),
	}

	// Track the instance
	migMutex.Lock()
	migInstances[deviceID] = append(migInstances[deviceID], *instance)
	migMutex.Unlock()

	return instance, nil
}

// DestroyMIGDevice destroys a MIG instance
func (m *Manager) DestroyMIGDevice(instanceID string) error {
	migMutex.Lock()
	defer migMutex.Unlock()

	for deviceID, instances := range migInstances {
		for i, instance := range instances {
			if instance.ID == instanceID {
				// Destroy compute instance
				if err := instance.ComputeInstance.Destroy(); err != nil {
					log.Printf("Failed to destroy compute instance: %v", err)
				}

				// Destroy GPU instance
				if err := instance.GPUInstance.Destroy(); err != nil {
					log.Printf("Failed to destroy GPU instance: %v", err)
				}

				// Remove from tracking
				migInstances[deviceID] = append(instances[:i], instances[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("MIG instance not found: %s", instanceID)
}

// GetMIGInstances returns all MIG instances for a device
func (m *Manager) GetMIGInstances(deviceID string) ([]MIGInstance, error) {
	migMutex.RLock()
	defer migMutex.RUnlock()

	instances, exists := migInstances[deviceID]
	if !exists {
		return nil, fmt.Errorf("no MIG instances found for device: %s", deviceID)
	}

	return instances, nil
}

// GetAllMIGInstances returns all MIG instances across all devices
func (m *Manager) GetAllMIGInstances() map[string][]MIGInstance {
	migMutex.RLock()
	defer migMutex.RUnlock()

	result := make(map[string][]MIGInstance)
	for deviceID, instances := range migInstances {
		result[deviceID] = make([]MIGInstance, len(instances))
		copy(result[deviceID], instances)
	}

	return result
}

// CleanupDevice removes all MIG instances from a device
func (m *Manager) CleanupDevice(deviceID string) error {
	device, err := m.findDevice(deviceID)
	if err != nil {
		return err
	}

	if err := CleanupMIGInstances(device.Handle); err != nil {
		return fmt.Errorf("failed to cleanup MIG instances: %v", err)
	}

	// Clear tracking
	migMutex.Lock()
	delete(migInstances, deviceID)
	migMutex.Unlock()

	return nil
}

// findDevice finds a device by UUID
func (m *Manager) findDevice(deviceID string) (GPUDeviceInfo, error) {
	for _, device := range m.devices {
		if device.UUID == deviceID {
			return device, nil
		}
	}
	return GPUDeviceInfo{}, fmt.Errorf("device not found: %s", deviceID)
}

// GetDeviceStatus returns the status of a specific device
func (m *Manager) GetDeviceStatus(deviceID string) (*types.GPUDeviceInfo, error) {
	device, err := m.findDevice(deviceID)
	if err != nil {
		return nil, err
	}

	return GetDeviceInfo(device.Handle)
}