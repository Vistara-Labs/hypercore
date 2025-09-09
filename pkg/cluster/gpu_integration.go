package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"vistara-node/pkg/gpu"
	pb "vistara-node/pkg/proto/cluster"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// GPUSpawnRequest represents a request to spawn a GPU model
type GPUSpawnRequest struct {
	ModelID     string            `json:"model_id"`
	ModelConfig *gpu.ModelConfig  `json:"model_config"`
	VRAMLimit   string            `json:"vram_limit"`   // e.g., "3GiB"
	CPUMemLimit string            `json:"cpu_mem_limit"` // e.g., "2GiB"
	SLOClass    string            `json:"slo_class"`    // interactive, batch
	GPUDevice   string            `json:"gpu_device"`   // GPU device ID
	Port        int               `json:"port"`         // Port for model server
}

// GPUSpawnResponse represents the response to a GPU spawn request
type GPUSpawnResponse struct {
	ModelID   string `json:"model_id"`
	Status    string `json:"status"`
	URL       string `json:"url"`
	VRAMUsed  int64  `json:"vram_used"`
	CPUMemUsed int64  `json:"cpu_mem_used"`
	Message   string `json:"message,omitempty"`
}

// GPUAgent extends the existing Agent with GPU capabilities
type GPUAgent struct {
	*Agent
	gpuScheduler *gpu.GPUScheduler
	gpuMetrics   *GPUMetrics
}

// GPUMetrics holds GPU-specific Prometheus metrics
type GPUMetrics struct {
	gpuModelCount        prometheus.Gauge
	gpuVRAMUtilization   prometheus.Gauge
	gpuCPUMemUtilization prometheus.Gauge
	gpuModelRequests     prometheus.Counter
	gpuModelErrors       prometheus.Counter
	gpuQuotaViolations   prometheus.Counter
	gpuModelRestarts     prometheus.Counter
}

// NewGPUAgent creates a new GPU-enabled agent
func NewGPUAgent(baseAgent *Agent, logger *log.Logger) (*GPUAgent, error) {
	// Create GPU scheduler
	gpuScheduler := gpu.NewGPUScheduler(logger)

	// Initialize GPU metrics
	gpuMetrics := &GPUMetrics{
		gpuModelCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hypercore_gpu_model_count",
			Help: "Number of active GPU models",
		}),
		gpuVRAMUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hypercore_gpu_vram_utilization_percent",
			Help: "GPU VRAM utilization percentage",
		}),
		gpuCPUMemUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hypercore_gpu_cpu_mem_utilization_percent",
			Help: "CPU memory utilization percentage",
		}),
		gpuModelRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hypercore_gpu_model_requests_total",
			Help: "Total number of GPU model requests",
		}),
		gpuModelErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hypercore_gpu_model_errors_total",
			Help: "Total number of GPU model errors",
		}),
		gpuQuotaViolations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hypercore_gpu_quota_violations_total",
			Help: "Total number of GPU quota violations",
		}),
		gpuModelRestarts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hypercore_gpu_model_restarts_total",
			Help: "Total number of GPU model restarts",
		}),
	}

	// Register GPU metrics
	prometheus.MustRegister(
		gpuMetrics.gpuModelCount,
		gpuMetrics.gpuVRAMUtilization,
		gpuMetrics.gpuCPUMemUtilization,
		gpuMetrics.gpuModelRequests,
		gpuMetrics.gpuModelErrors,
		gpuMetrics.gpuQuotaViolations,
		gpuMetrics.gpuModelRestarts,
	)

	return &GPUAgent{
		Agent:        baseAgent,
		gpuScheduler: gpuScheduler,
		gpuMetrics:   gpuMetrics,
	}, nil
}

// SpawnGPUModel spawns a GPU model with quotas and isolation
func (ga *GPUAgent) SpawnGPUModel(ctx context.Context, req *GPUSpawnRequest) (*GPUSpawnResponse, error) {
	ga.logger.Infof("Spawning GPU model %s with VRAM limit %s", req.ModelID, req.VRAMLimit)

	// Convert request to GPU shim config
	shimConfig, err := ga.convertToGPUShimConfig(req)
	if err != nil {
		ga.gpuMetrics.gpuModelErrors.Inc()
		return nil, fmt.Errorf("failed to convert request to GPU config: %w", err)
	}

	// Spawn the model
	err = ga.gpuScheduler.SpawnModel(ctx, req.ModelID, shimConfig)
	if err != nil {
		ga.gpuMetrics.gpuModelErrors.Inc()
		return nil, fmt.Errorf("failed to spawn GPU model: %w", err)
	}

	// Get model instance for response
	instance, err := ga.gpuScheduler.GetModel(req.ModelID)
	if err != nil {
		ga.gpuMetrics.gpuModelErrors.Inc()
		return nil, fmt.Errorf("failed to get model instance: %w", err)
	}

	// Generate URL for the model
	url := ga.generateModelURL(req.ModelID, req.Port)

	response := &GPUSpawnResponse{
		ModelID:    req.ModelID,
		Status:     "running",
		URL:        url,
		VRAMUsed:   instance.VRAMUsed,
		CPUMemUsed: instance.CPUMemUsed,
		Message:    "GPU model spawned successfully",
	}

	ga.gpuMetrics.gpuModelCount.Inc()
	ga.logger.Infof("Successfully spawned GPU model %s at %s", req.ModelID, url)

	return response, nil
}

// StopGPUModel stops a GPU model
func (ga *GPUAgent) StopGPUModel(modelID string) error {
	ga.logger.Infof("Stopping GPU model %s", modelID)

	err := ga.gpuScheduler.StopModel(modelID)
	if err != nil {
		ga.gpuMetrics.gpuModelErrors.Inc()
		return fmt.Errorf("failed to stop GPU model: %w", err)
	}

	ga.gpuMetrics.gpuModelCount.Dec()
	ga.logger.Infof("Successfully stopped GPU model %s", modelID)

	return nil
}

// ListGPUModels returns all active GPU models
func (ga *GPUAgent) ListGPUModels() []*gpu.ModelInstance {
	return ga.gpuScheduler.ListModels()
}

// GetGPUModel returns a specific GPU model
func (ga *GPUAgent) GetGPUModel(modelID string) (*gpu.ModelInstance, error) {
	return ga.gpuScheduler.GetModel(modelID)
}

// RecordGPUModelRequest records a request to a GPU model
func (ga *GPUAgent) RecordGPUModelRequest(modelID string) {
	ga.gpuScheduler.RecordRequest(modelID)
	ga.gpuMetrics.gpuModelRequests.Inc()
}

// RecordGPUModelError records an error for a GPU model
func (ga *GPUAgent) RecordGPUModelError(modelID string) {
	ga.gpuScheduler.RecordError(modelID)
	ga.gpuMetrics.gpuModelErrors.Inc()
}

// RecordGPUQuotaViolation records a quota violation
func (ga *GPUAgent) RecordGPUQuotaViolation(modelID string) {
	ga.gpuScheduler.RecordQuotaViolation(modelID)
	ga.gpuMetrics.gpuQuotaViolations.Inc()
}

// convertToGPUShimConfig converts a spawn request to GPU shim config
func (ga *GPUAgent) convertToGPUShimConfig(req *GPUSpawnRequest) (*gpu.GPUShimConfig, error) {
	// Parse VRAM limit
	vramBytes, err := parseMemorySize(req.VRAMLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid VRAM limit %s: %w", req.VRAMLimit, err)
	}

	// Parse CPU memory limit
	cpuMemBytes, err := parseMemorySize(req.CPUMemLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU memory limit %s: %w", req.CPUMemLimit, err)
	}

	// Convert to MB
	cpuMemMB := int(cpuMemBytes / (1024 * 1024))

	// Set defaults
	if req.Port == 0 {
		req.Port = 8080
	}
	if req.GPUDevice == "" {
		req.GPUDevice = "0"
	}
	if req.SLOClass == "" {
		req.SLOClass = "interactive"
	}

	return &gpu.GPUShimConfig{
		VRAMLimitBytes:    vramBytes,
		CPUMemLimitMB:     cpuMemMB,
		CUDAVisibleDevices: req.GPUDevice,
		ModelEngine:       req.ModelConfig.Engine,
		ModelPath:         req.ModelConfig.ModelPath,
		ModelArgs:         req.ModelConfig.ModelArgs,
		Port:              req.Port,
	}, nil
}

// generateModelURL generates a URL for the model
func (ga *GPUAgent) generateModelURL(modelID string, port int) string {
	// Use the same domain generation logic as regular containers
	// but with a different prefix for GPU models
	shortDomain := ga.generateShortDomain(modelID)
	// Replace "app-" with "gpu-" for GPU models
	gpuDomain := strings.Replace(shortDomain, "app-", "gpu-", 1)
	return fmt.Sprintf("http://%s:%d", gpuDomain, port)
}

// parseMemorySize parses memory size strings like "3GiB", "2048MB"
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

// UpdateGPUMetrics updates GPU-specific metrics
func (ga *GPUAgent) UpdateGPUMetrics() {
	models := ga.gpuScheduler.ListModels()
	
	// Update model count
	ga.gpuMetrics.gpuModelCount.Set(float64(len(models)))

	// Calculate total VRAM and CPU memory utilization
	var totalVRAMUsed, totalVRAMLimit int64
	var totalCPUMemUsed, totalCPUMemLimit int64

	for _, model := range models {
		totalVRAMUsed += model.VRAMUsed
		totalVRAMLimit += model.Config.VRAMLimitBytes
		totalCPUMemUsed += model.CPUMemUsed
		totalCPUMemLimit += int64(model.Config.CPUMemLimitMB * 1024 * 1024)
	}

	// Update utilization metrics
	if totalVRAMLimit > 0 {
		vramUtilization := float64(totalVRAMUsed) / float64(totalVRAMLimit) * 100
		ga.gpuMetrics.gpuVRAMUtilization.Set(vramUtilization)
	}

	if totalCPUMemLimit > 0 {
		cpuMemUtilization := float64(totalCPUMemUsed) / float64(totalCPUMemLimit) * 100
		ga.gpuMetrics.gpuCPUMemUtilization.Set(cpuMemUtilization)
	}
}