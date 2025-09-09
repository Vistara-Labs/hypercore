package gpu

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// GPUShimConfig represents configuration for GPU model execution
type GPUShimConfig struct {
	VRAMLimitBytes     int64    `json:"vram_limit_bytes"`     // VRAM quota in bytes
	CPUMemLimitMB      int      `json:"cpu_mem_limit_mb"`     // CPU memory limit in MB
	CUDAVisibleDevices string   `json:"cuda_visible_devices"` // GPU device IDs
	ModelEngine        string   `json:"model_engine"`         // python3, vllm, etc.
	ModelPath          string   `json:"model_path"`           // Path to model script/binary
	ModelArgs          []string `json:"model_args"`           // Additional args for model
	Port               int      `json:"port"`                 // Port for model server
}

// GPUShim handles GPU model execution with quotas and isolation
type GPUShim struct {
	config *GPUShimConfig
	logger *log.Logger
}

// NewGPUShim creates a new GPU shim instance
func NewGPUShim(config *GPUShimConfig, logger *log.Logger) *GPUShim {
	return &GPUShim{
		config: config,
		logger: logger,
	}
}

// SpawnModel executes a model with GPU quotas and isolation
func (g *GPUShim) SpawnModel(ctx context.Context) (*exec.Cmd, error) {
	// Create sandbox command
	sandboxCmd := exec.CommandContext(ctx, "/opt/hypercore/bin/sandbox", g.config.ModelEngine, g.config.ModelPath)
	sandboxCmd.Args = append(sandboxCmd.Args, g.config.ModelArgs...)

	// Set environment variables for GPU quotas
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("HYPERCORE_VRAM_LIMIT_BYTES=%d", g.config.VRAMLimitBytes),
		fmt.Sprintf("HYPERCORE_CPU_MEM_MB=%d", g.config.CPUMemLimitMB),
		fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", g.config.CUDAVisibleDevices),
		"PYTHONUNBUFFERED=1",
	)

	sandboxCmd.Env = env
	sandboxCmd.Stdout = os.Stdout
	sandboxCmd.Stderr = os.Stderr
	sandboxCmd.Stdin = os.Stdin

	g.logger.Infof("Spawning GPU model with VRAM limit: %d bytes, CPU limit: %d MB",
		g.config.VRAMLimitBytes, g.config.CPUMemLimitMB)

	return sandboxCmd, nil
}

// ValidateConfig validates the GPU shim configuration
func (g *GPUShim) ValidateConfig() error {
	if g.config.VRAMLimitBytes <= 0 {
		return fmt.Errorf("VRAM limit must be positive")
	}

	if g.config.CPUMemLimitMB <= 0 {
		return fmt.Errorf("CPU memory limit must be positive")
	}

	if g.config.ModelEngine == "" {
		return fmt.Errorf("model engine must be specified")
	}

	if g.config.ModelPath == "" {
		return fmt.Errorf("model path must be specified")
	}

	// Check if sandbox binary exists
	if _, err := os.Stat("/opt/hypercore/bin/sandbox"); os.IsNotExist(err) {
		return fmt.Errorf("sandbox binary not found at /opt/hypercore/bin/sandbox")
	}

	// Check if CUDA shim exists
	if _, err := os.Stat("/opt/hypercore/libhypercuda.so"); os.IsNotExist(err) {
		return fmt.Errorf("CUDA shim not found at /opt/hypercore/libhypercuda.so")
	}

	return nil
}

// GetGPUInfo returns current GPU information
func (g *GPUShim) GetGPUInfo() (map[string]interface{}, error) {
	// This would query nvidia-smi or similar
	// For now, return placeholder info
	return map[string]interface{}{
		"cuda_visible_devices": g.config.CUDAVisibleDevices,
		"vram_limit_bytes":     g.config.VRAMLimitBytes,
		"cpu_mem_limit_mb":     g.config.CPUMemLimitMB,
	}, nil
}
