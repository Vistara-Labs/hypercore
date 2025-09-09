package gpu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModelConfig represents the configuration for a GPU model
type ModelConfig struct {
	Name        string   `json:"name"`          // Model name
	Engine      string   `json:"engine"`        // python3, vllm, etc.
	ModelPath   string   `json:"model_path"`    // Path to model script/binary
	ModelArgs   []string `json:"model_args"`    // Additional args for model
	Weights     string   `json:"weights"`       // Path to model weights
	VRAMLimit   string   `json:"vram_limit"`    // VRAM limit (e.g., "3GiB", "2048MB")
	CPUMemLimit string   `json:"cpu_mem_limit"` // CPU memory limit (e.g., "2GiB", "1024MB")
	SLOClass    string   `json:"slo_class"`     // interactive, batch, etc.
	BatchMax    int      `json:"batch_max"`     // Maximum batch size
	Port        int      `json:"port"`          // Port for model server
	GPUDevice   string   `json:"gpu_device"`    // GPU device ID (e.g., "0", "1")
}

// LoadModelConfig loads a model configuration from a YAML/JSON file
func LoadModelConfig(configPath string) (*ModelConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config ModelConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate required fields
	if config.Name == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if config.Engine == "" {
		return nil, fmt.Errorf("model engine is required")
	}
	if config.ModelPath == "" {
		return nil, fmt.Errorf("model path is required")
	}
	if config.VRAMLimit == "" {
		return nil, fmt.Errorf("VRAM limit is required")
	}
	if config.CPUMemLimit == "" {
		return nil, fmt.Errorf("CPU memory limit is required")
	}

	// Set defaults
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.GPUDevice == "" {
		config.GPUDevice = "0"
	}
	if config.SLOClass == "" {
		config.SLOClass = "interactive"
	}
	if config.BatchMax == 0 {
		config.BatchMax = 1
	}

	return &config, nil
}

// ToGPUShimConfig converts ModelConfig to GPUShimConfig
func (mc *ModelConfig) ToGPUShimConfig() (*GPUShimConfig, error) {
	// Parse VRAM limit
	vramBytes, err := parseMemorySize(mc.VRAMLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid VRAM limit %s: %w", mc.VRAMLimit, err)
	}

	// Parse CPU memory limit
	cpuMemBytes, err := parseMemorySize(mc.CPUMemLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU memory limit %s: %w", mc.CPUMemLimit, err)
	}

	// Convert to MB
	cpuMemMB := int(cpuMemBytes / (1024 * 1024))

	return &GPUShimConfig{
		VRAMLimitBytes:     vramBytes,
		CPUMemLimitMB:      cpuMemMB,
		CUDAVisibleDevices: mc.GPUDevice,
		ModelEngine:        mc.Engine,
		ModelPath:          mc.ModelPath,
		ModelArgs:          mc.ModelArgs,
		Port:               mc.Port,
	}, nil
}

// parseMemorySize parses memory size strings like "3GiB", "2048MB", "2GB"
func parseMemorySize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Remove spaces and convert to lowercase
	sizeStr = strings.TrimSpace(strings.ToLower(sizeStr))

	var multiplier int64 = 1
	var size int64

	// Parse the number part
	var i int
	for i = 0; i < len(sizeStr); i++ {
		if sizeStr[i] < '0' || sizeStr[i] > '9' {
			break
		}
	}

	if i == 0 {
		return 0, fmt.Errorf("no number found in size string")
	}

	if _, err := fmt.Sscanf(sizeStr[:i], "%d", &size); err != nil {
		return 0, fmt.Errorf("invalid number in size string")
	}

	// Parse the unit part
	unit := sizeStr[i:]
	switch unit {
	case "b", "bytes":
		multiplier = 1
	case "kb", "kib":
		multiplier = 1024
	case "mb", "mib":
		multiplier = 1024 * 1024
	case "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return size * multiplier, nil
}

// SaveModelConfig saves a model configuration to a file
func SaveModelConfig(config *ModelConfig, configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ExampleModelConfig returns an example model configuration
func ExampleModelConfig() *ModelConfig {
	return &ModelConfig{
		Name:        "llama-7b-chat",
		Engine:      "python3",
		ModelPath:   "/opt/models/llama-7b-chat/server.py",
		ModelArgs:   []string{"--model", "llama-7b-chat", "--port", "8080"},
		Weights:     "/opt/models/llama-7b-chat/weights",
		VRAMLimit:   "3GiB",
		CPUMemLimit: "2GiB",
		SLOClass:    "interactive",
		BatchMax:    1,
		Port:        8080,
		GPUDevice:   "0",
	}
}
