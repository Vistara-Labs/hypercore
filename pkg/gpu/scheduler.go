package gpu

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// ModelState represents the state of a model instance
type ModelState int

const (
	ModelStateStopped ModelState = iota
	ModelStateStarting
	ModelStateRunning
	ModelStateStopping
	ModelStateError
)

func (s ModelState) String() string {
	switch s {
	case ModelStateStopped:
		return "stopped"
	case ModelStateStarting:
		return "starting"
	case ModelStateRunning:
		return "running"
	case ModelStateStopping:
		return "stopping"
	case ModelStateError:
		return "error"
	default:
		return "unknown"
	}
}

// ModelInstance represents a running model instance
type ModelInstance struct {
	ID           string         `json:"id"`
	Config       *GPUShimConfig `json:"config"`
	State        ModelState     `json:"state"`
	Process      *exec.Cmd      `json:"-"`
	StartedAt    time.Time      `json:"started_at"`
	LastRequest  time.Time      `json:"last_request"`
	RequestCount int64          `json:"request_count"`
	ErrorCount   int64          `json:"error_count"`
	VRAMUsed     int64          `json:"vram_used"`
	CPUMemUsed   int64          `json:"cpu_mem_used"`
}

// GPUScheduler manages GPU model instances with quotas and scheduling
type GPUScheduler struct {
	models      map[string]*ModelInstance
	modelsMutex sync.RWMutex
	shim        *GPUShim
	logger      *log.Logger

	// Prometheus metrics
	modelCount        prometheus.Gauge
	vramUtilization   prometheus.Gauge
	cpuMemUtilization prometheus.Gauge
	modelRequests     prometheus.Counter
	modelErrors       prometheus.Counter
	modelRestarts     prometheus.Counter
	quotaViolations   prometheus.Counter
}

// NewGPUScheduler creates a new GPU scheduler
func NewGPUScheduler(logger *log.Logger) *GPUScheduler {
	// Initialize Prometheus metrics
	modelCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "hypercore_gpu_model_count",
		Help: "Number of active GPU models",
	})
	vramUtilization := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "hypercore_gpu_vram_utilization_percent",
		Help: "GPU VRAM utilization percentage",
	})
	cpuMemUtilization := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "hypercore_gpu_cpu_mem_utilization_percent",
		Help: "CPU memory utilization percentage",
	})
	modelRequests := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hypercore_gpu_model_requests_total",
		Help: "Total number of model requests",
	})
	modelErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hypercore_gpu_model_errors_total",
		Help: "Total number of model errors",
	})
	modelRestarts := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hypercore_gpu_model_restarts_total",
		Help: "Total number of model restarts",
	})
	quotaViolations := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hypercore_gpu_quota_violations_total",
		Help: "Total number of quota violations",
	})

	// Register metrics
	prometheus.MustRegister(modelCount, vramUtilization, cpuMemUtilization,
		modelRequests, modelErrors, modelRestarts, quotaViolations)

	return &GPUScheduler{
		models:            make(map[string]*ModelInstance),
		logger:            logger,
		modelCount:        modelCount,
		vramUtilization:   vramUtilization,
		cpuMemUtilization: cpuMemUtilization,
		modelRequests:     modelRequests,
		modelErrors:       modelErrors,
		modelRestarts:     modelRestarts,
		quotaViolations:   quotaViolations,
	}
}

// SpawnModel starts a new model instance
func (s *GPUScheduler) SpawnModel(ctx context.Context, modelID string, config *GPUShimConfig) error {
	s.modelsMutex.Lock()
	defer s.modelsMutex.Unlock()

	// Check if model already exists
	if _, exists := s.models[modelID]; exists {
		return fmt.Errorf("model %s already exists", modelID)
	}

	// Create shim for this model
	shim := NewGPUShim(config, s.logger)
	if err := shim.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid config for model %s: %w", modelID, err)
	}

	// Create model instance
	instance := &ModelInstance{
		ID:        modelID,
		Config:    config,
		State:     ModelStateStarting,
		StartedAt: time.Now(),
	}

	// Spawn the model process
	cmd, err := shim.SpawnModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to spawn model %s: %w", modelID, err)
	}

	instance.Process = cmd
	s.models[modelID] = instance

	// Start the process
	if err := cmd.Start(); err != nil {
		instance.State = ModelStateError
		s.modelErrors.Inc()
		return fmt.Errorf("failed to start model %s: %w", modelID, err)
	}

	instance.State = ModelStateRunning
	s.modelCount.Inc()

	s.logger.Infof("Successfully spawned GPU model %s with VRAM limit %d bytes",
		modelID, config.VRAMLimitBytes)

	// Start monitoring goroutine
	go s.monitorModel(instance)

	return nil
}

// StopModel stops a model instance
func (s *GPUScheduler) StopModel(modelID string) error {
	s.modelsMutex.Lock()
	defer s.modelsMutex.Unlock()

	instance, exists := s.models[modelID]
	if !exists {
		return fmt.Errorf("model %s not found", modelID)
	}

	if instance.State == ModelStateStopped {
		return nil
	}

	instance.State = ModelStateStopping

	// Kill the process
	if instance.Process != nil && instance.Process.Process != nil {
		if err := instance.Process.Process.Kill(); err != nil {
			s.logger.Warnf("Failed to kill model %s: %v", modelID, err)
		}
	}

	instance.State = ModelStateStopped
	delete(s.models, modelID)
	s.modelCount.Dec()

	s.logger.Infof("Stopped GPU model %s", modelID)
	return nil
}

// GetModel returns a model instance
func (s *GPUScheduler) GetModel(modelID string) (*ModelInstance, error) {
	s.modelsMutex.RLock()
	defer s.modelsMutex.RUnlock()

	instance, exists := s.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not found", modelID)
	}

	return instance, nil
}

// ListModels returns all model instances
func (s *GPUScheduler) ListModels() []*ModelInstance {
	s.modelsMutex.RLock()
	defer s.modelsMutex.RUnlock()

	models := make([]*ModelInstance, 0, len(s.models))
	for _, model := range s.models {
		models = append(models, model)
	}

	return models
}

// monitorModel monitors a model instance for health and metrics
func (s *GPUScheduler) monitorModel(instance *ModelInstance) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if process is still running
			if instance.Process != nil && instance.Process.Process != nil {
				if err := instance.Process.Process.Signal(syscall.Signal(0)); err != nil {
					// Process is dead
					s.logger.Warnf("Model %s process died, restarting...", instance.ID)
					s.modelRestarts.Inc()

					// Restart the model
					ctx := context.Background()
					cmd, err := s.shim.SpawnModel(ctx)
					if err != nil {
						s.logger.Errorf("Failed to restart model %s: %v", instance.ID, err)
						instance.State = ModelStateError
						return
					}

					instance.Process = cmd
					if err := cmd.Start(); err != nil {
						s.logger.Errorf("Failed to start restarted model %s: %v", instance.ID, err)
						instance.State = ModelStateError
						return
					}

					instance.State = ModelStateRunning
					instance.StartedAt = time.Now()
				}
			}

			// Update metrics
			s.updateModelMetrics(instance)
		}
	}
}

// updateModelMetrics updates Prometheus metrics for a model
func (s *GPUScheduler) updateModelMetrics(instance *ModelInstance) {
	// Update VRAM utilization
	if instance.Config.VRAMLimitBytes > 0 {
		utilization := float64(instance.VRAMUsed) / float64(instance.Config.VRAMLimitBytes) * 100
		s.vramUtilization.Set(utilization)
	}

	// Update CPU memory utilization
	if instance.Config.CPUMemLimitMB > 0 {
		utilization := float64(instance.CPUMemUsed) / float64(instance.Config.CPUMemLimitMB*1024*1024) * 100
		s.cpuMemUtilization.Set(utilization)
	}
}

// RecordRequest records a request to a model
func (s *GPUScheduler) RecordRequest(modelID string) {
	s.modelsMutex.Lock()
	defer s.modelsMutex.Unlock()

	if instance, exists := s.models[modelID]; exists {
		instance.LastRequest = time.Now()
		instance.RequestCount++
		s.modelRequests.Inc()
	}
}

// RecordError records an error for a model
func (s *GPUScheduler) RecordError(modelID string) {
	s.modelsMutex.Lock()
	defer s.modelsMutex.Unlock()

	if instance, exists := s.models[modelID]; exists {
		instance.ErrorCount++
		s.modelErrors.Inc()
	}
}

// RecordQuotaViolation records a quota violation
func (s *GPUScheduler) RecordQuotaViolation(modelID string) {
	s.modelsMutex.Lock()
	defer s.modelsMutex.Unlock()

	if instance, exists := s.models[modelID]; exists {
		s.logger.Warnf("Quota violation for model %s", modelID)
		s.quotaViolations.Inc()

		// Mark model for restart
		instance.State = ModelStateError
	}
}
