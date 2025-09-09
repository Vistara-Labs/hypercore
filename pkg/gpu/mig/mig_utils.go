package mig

import (
	"fmt"

	"github.com/NVIDIA/go-nvml"
)

// EnableMIGMode enables MIG mode on the specified device
func EnableMIGMode(device nvml.Device) error {
	mode, err := device.GetMigMode()
	if err != nil {
		return fmt.Errorf("failed to get MIG mode: %v", err)
	}

	// Check if MIG mode is already enabled
	if mode.Current == nvml.DEVICE_MIG_ENABLE {
		return nil
	}

	// Enable MIG mode
	if err := device.SetMigMode(nvml.DEVICE_MIG_ENABLE); err != nil {
		return fmt.Errorf("failed to enable MIG mode: %v", err)
	}

	return nil
}

// DisableMIGMode disables MIG mode on the specified device
func DisableMIGMode(device nvml.Device) error {
	mode, err := device.GetMigMode()
	if err != nil {
		return fmt.Errorf("failed to get MIG mode: %v", err)
	}

	// Check if MIG mode is already disabled
	if mode.Current == nvml.DEVICE_MIG_DISABLE {
		return nil
	}

	// Disable MIG mode
	if err := device.SetMigMode(nvml.DEVICE_MIG_DISABLE); err != nil {
		return fmt.Errorf("failed to disable MIG mode: %v", err)
	}

	return nil
}

// GetMIGMode returns the current MIG mode status
func GetMIGMode(device nvml.Device) (nvml.MigMode, error) {
	mode, err := device.GetMigMode()
	if err != nil {
		return nvml.MigMode{}, fmt.Errorf("failed to get MIG mode: %v", err)
	}
	return mode, nil
}

// GetGPUInstanceProfiles returns available GPU instance profiles
func GetGPUInstanceProfiles(device nvml.Device) ([]nvml.GpuInstanceProfileInfo, error) {
	profiles, err := device.GetGpuInstanceProfileInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get GPU instance profiles: %v", err)
	}
	return profiles, nil
}

// GetComputeInstanceProfiles returns available compute instance profiles
func GetComputeInstanceProfiles(gi nvml.GpuInstance) ([]nvml.ComputeInstanceProfileInfo, error) {
	profiles, err := gi.GetComputeInstanceProfileInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get compute instance profiles: %v", err)
	}
	return profiles, nil
}

// CleanupMIGInstances removes all MIG instances from a device
func CleanupMIGInstances(device nvml.Device) error {
	// Get all GPU instances
	gpuInstances, err := device.GetGpuInstances()
	if err != nil {
		return fmt.Errorf("failed to get GPU instances: %v", err)
	}

	// Destroy all GPU instances
	for _, gi := range gpuInstances {
		if err := gi.Destroy(); err != nil {
			log.Printf("Failed to destroy GPU instance: %v", err)
		}
	}

	return nil
}