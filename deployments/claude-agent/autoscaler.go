package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

// AutoScaler manages automatic scaling of Claude agents based on load
type AutoScaler struct {
	manager          *AgentManager
	prometheusAPI    v1.API
	logger           *logrus.Logger
	config           *AutoScalerConfig
	mutex            sync.RWMutex
	isRunning        bool
	stopChan         chan struct{}
	lastScaleTime    time.Time
	scaleUpCooldown  time.Duration
	scaleDownCooldown time.Duration
}

// AutoScalerConfig configuration for autoscaler
type AutoScalerConfig struct {
	MinAgents                int
	MaxAgents                int
	TargetSessionsPerAgent   int
	TargetCPUPercent         float64
	TargetMemoryPercent      float64
	ScaleUpThreshold         float64
	ScaleDownThreshold       float64
	EvaluationInterval       time.Duration
	ScaleUpCooldownPeriod    time.Duration
	ScaleDownCooldownPeriod  time.Duration
	AggressiveScaling        bool
	PredictiveScaling        bool
}

// DefaultAutoScalerConfig returns default configuration for 10k users
func DefaultAutoScalerConfig() *AutoScalerConfig {
	return &AutoScalerConfig{
		MinAgents:                10,   // Minimum agents always running
		MaxAgents:                1000, // Maximum for 10k concurrent users
		TargetSessionsPerAgent:   100,  // Each agent handles 100 sessions
		TargetCPUPercent:         70.0,
		TargetMemoryPercent:      80.0,
		ScaleUpThreshold:         0.80, // Scale up at 80% capacity
		ScaleDownThreshold:       0.30, // Scale down at 30% capacity
		EvaluationInterval:       30 * time.Second,
		ScaleUpCooldownPeriod:    2 * time.Minute,
		ScaleDownCooldownPeriod:  5 * time.Minute,
		AggressiveScaling:        true,  // Fast scale-up for 10k users
		PredictiveScaling:        true,  // Predict load based on trends
	}
}

// ScalingDecision represents a scaling decision
type ScalingDecision struct {
	Action           string  // "scale_up", "scale_down", "none"
	CurrentAgents    int
	DesiredAgents    int
	Reason           string
	Metrics          map[string]float64
	Timestamp        time.Time
}

// NewAutoScaler creates a new autoscaler
func NewAutoScaler(manager *AgentManager, prometheusURL string, config *AutoScalerConfig) (*AutoScaler, error) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Create Prometheus API client
	promClient, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &AutoScaler{
		manager:           manager,
		prometheusAPI:     v1.NewAPI(promClient),
		logger:            logger,
		config:            config,
		isRunning:         false,
		stopChan:          make(chan struct{}),
		lastScaleTime:     time.Now(),
		scaleUpCooldown:   config.ScaleUpCooldownPeriod,
		scaleDownCooldown: config.ScaleDownCooldownPeriod,
	}, nil
}

// Start starts the autoscaler
func (as *AutoScaler) Start(ctx context.Context) {
	as.mutex.Lock()
	if as.isRunning {
		as.mutex.Unlock()
		return
	}
	as.isRunning = true
	as.mutex.Unlock()

	as.logger.WithFields(logrus.Fields{
		"min_agents":     as.config.MinAgents,
		"max_agents":     as.config.MaxAgents,
		"eval_interval":  as.config.EvaluationInterval,
		"aggressive":     as.config.AggressiveScaling,
		"predictive":     as.config.PredictiveScaling,
	}).Info("Starting autoscaler")

	// Ensure minimum agents are running
	go as.ensureMinimumAgents(ctx)

	// Start evaluation loop
	go as.evaluationLoop(ctx)
}

// Stop stops the autoscaler
func (as *AutoScaler) Stop() {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	if !as.isRunning {
		return
	}

	as.logger.Info("Stopping autoscaler")
	close(as.stopChan)
	as.isRunning = false
}

// evaluationLoop continuously evaluates metrics and makes scaling decisions
func (as *AutoScaler) evaluationLoop(ctx context.Context) {
	ticker := time.NewTicker(as.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			as.logger.Info("Evaluation loop stopped: context cancelled")
			return
		case <-as.stopChan:
			as.logger.Info("Evaluation loop stopped")
			return
		case <-ticker.C:
			if err := as.evaluate(ctx); err != nil {
				as.logger.WithError(err).Error("Evaluation failed")
			}
		}
	}
}

// evaluate evaluates current metrics and makes scaling decision
func (as *AutoScaler) evaluate(ctx context.Context) error {
	// Collect metrics
	metrics, err := as.collectMetrics(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Make scaling decision
	decision := as.makeScalingDecision(metrics)

	// Log decision
	as.logger.WithFields(logrus.Fields{
		"action":          decision.Action,
		"current_agents":  decision.CurrentAgents,
		"desired_agents":  decision.DesiredAgents,
		"reason":          decision.Reason,
		"active_sessions": metrics["active_sessions"],
		"cpu_percent":     metrics["avg_cpu_percent"],
		"memory_percent":  metrics["avg_memory_percent"],
	}).Info("Scaling evaluation completed")

	// Execute scaling action
	if decision.Action != "none" {
		if err := as.executeScaling(ctx, decision); err != nil {
			return fmt.Errorf("failed to execute scaling: %w", err)
		}
	}

	return nil
}

// collectMetrics collects metrics from Prometheus
func (as *AutoScaler) collectMetrics(ctx context.Context) (map[string]float64, error) {
	metrics := make(map[string]float64)

	// Query active sessions
	activeSessions, err := as.queryMetric(ctx, "agent_active_sessions")
	if err != nil {
		return nil, err
	}
	metrics["active_sessions"] = activeSessions

	// Query current agent count
	agentCount, err := as.queryMetric(ctx, "hypercore_agent_active_count")
	if err != nil {
		return nil, err
	}
	metrics["current_agents"] = agentCount

	// Query request rate
	requestRate, err := as.queryMetric(ctx, "rate(agent_requests_total[5m])")
	if err == nil {
		metrics["request_rate"] = requestRate
	}

	// Query error rate
	errorRate, err := as.queryMetric(ctx, "rate(agent_errors_total[5m])")
	if err == nil {
		metrics["error_rate"] = errorRate
	}

	// Query average CPU usage
	avgCPU, err := as.queryMetric(ctx, "avg(rate(container_cpu_usage_seconds_total{container=\"claude-agent\"}[5m])) * 100")
	if err == nil {
		metrics["avg_cpu_percent"] = avgCPU
	}

	// Query average memory usage
	avgMemory, err := as.queryMetric(ctx, "avg(container_memory_usage_bytes{container=\"claude-agent\"} / container_spec_memory_limit_bytes{container=\"claude-agent\"}) * 100")
	if err == nil {
		metrics["avg_memory_percent"] = avgMemory
	}

	// Query p95 latency
	p95Latency, err := as.queryMetric(ctx, "histogram_quantile(0.95, rate(agent_request_duration_seconds_bucket[5m]))")
	if err == nil {
		metrics["p95_latency_seconds"] = p95Latency
	}

	return metrics, nil
}

// makeScalingDecision makes a scaling decision based on metrics
func (as *AutoScaler) makeScalingDecision(metrics map[string]float64) *ScalingDecision {
	currentAgents := int(metrics["current_agents"])
	activeSessions := metrics["active_sessions"]

	decision := &ScalingDecision{
		Action:        "none",
		CurrentAgents: currentAgents,
		DesiredAgents: currentAgents,
		Metrics:       metrics,
		Timestamp:     time.Now(),
	}

	// Calculate capacity utilization
	targetTotalSessions := float64(currentAgents * as.config.TargetSessionsPerAgent)
	utilization := 0.0
	if targetTotalSessions > 0 {
		utilization = activeSessions / targetTotalSessions
	}

	// Check cooldown periods
	timeSinceLastScale := time.Since(as.lastScaleTime)

	// SCALE UP conditions
	if utilization > as.config.ScaleUpThreshold && currentAgents < as.config.MaxAgents {
		if timeSinceLastScale < as.scaleUpCooldown {
			decision.Reason = fmt.Sprintf("Scale up needed but in cooldown period (%v remaining)", as.scaleUpCooldown-timeSinceLastScale)
			return decision
		}

		// Calculate desired agents
		desiredAgents := int(math.Ceil(activeSessions / float64(as.config.TargetSessionsPerAgent)))

		// Aggressive scaling: scale faster for high load
		if as.config.AggressiveScaling && utilization > 0.90 {
			desiredAgents = int(math.Ceil(float64(desiredAgents) * 1.5))
		}

		// Predictive scaling: add buffer for predicted growth
		if as.config.PredictiveScaling {
			requestRate := metrics["request_rate"]
			if requestRate > 0 {
				// Add 20% buffer based on request rate trend
				desiredAgents = int(math.Ceil(float64(desiredAgents) * 1.2))
			}
		}

		// Enforce max limit
		if desiredAgents > as.config.MaxAgents {
			desiredAgents = as.config.MaxAgents
		}

		// Ensure we scale by at least 1
		if desiredAgents <= currentAgents {
			desiredAgents = currentAgents + 1
		}

		decision.Action = "scale_up"
		decision.DesiredAgents = desiredAgents
		decision.Reason = fmt.Sprintf("High utilization: %.2f%% (threshold: %.2f%%)", utilization*100, as.config.ScaleUpThreshold*100)
		return decision
	}

	// SCALE DOWN conditions
	if utilization < as.config.ScaleDownThreshold && currentAgents > as.config.MinAgents {
		if timeSinceLastScale < as.scaleDownCooldown {
			decision.Reason = fmt.Sprintf("Scale down possible but in cooldown period (%v remaining)", as.scaleDownCooldown-timeSinceLastScale)
			return decision
		}

		// Calculate desired agents (more conservative)
		desiredAgents := int(math.Ceil(activeSessions / float64(as.config.TargetSessionsPerAgent)))

		// Add safety buffer (don't scale down too aggressively)
		desiredAgents = int(math.Ceil(float64(desiredAgents) * 1.2))

		// Enforce min limit
		if desiredAgents < as.config.MinAgents {
			desiredAgents = as.config.MinAgents
		}

		// Only scale down if significant reduction
		if currentAgents-desiredAgents < 2 {
			decision.Reason = "Scale down not significant enough"
			return decision
		}

		decision.Action = "scale_down"
		decision.DesiredAgents = desiredAgents
		decision.Reason = fmt.Sprintf("Low utilization: %.2f%% (threshold: %.2f%%)", utilization*100, as.config.ScaleDownThreshold*100)
		return decision
	}

	// Check resource-based scaling (CPU/Memory)
	if metrics["avg_cpu_percent"] > as.config.TargetCPUPercent || metrics["avg_memory_percent"] > as.config.TargetMemoryPercent {
		if currentAgents < as.config.MaxAgents {
			decision.Action = "scale_up"
			decision.DesiredAgents = currentAgents + int(math.Ceil(float64(currentAgents)*0.2)) // Scale up by 20%
			decision.Reason = fmt.Sprintf("High resource usage: CPU=%.1f%%, Memory=%.1f%%", metrics["avg_cpu_percent"], metrics["avg_memory_percent"])
			return decision
		}
	}

	decision.Reason = fmt.Sprintf("Within acceptable range: utilization=%.2f%%", utilization*100)
	return decision
}

// executeScaling executes the scaling decision
func (as *AutoScaler) executeScaling(ctx context.Context, decision *ScalingDecision) error {
	delta := decision.DesiredAgents - decision.CurrentAgents

	as.logger.WithFields(logrus.Fields{
		"action":       decision.Action,
		"current":      decision.CurrentAgents,
		"desired":      decision.DesiredAgents,
		"delta":        delta,
	}).Info("Executing scaling action")

	if decision.Action == "scale_up" {
		// Spawn new agents
		for i := 0; i < delta; i++ {
			req := &AgentSpawnRequest{
				UserID:        "autoscaler",
				Cores:         4,
				Memory:        8192,
				MaxConcurrent: 100,
				Timeout:       300,
			}

			if _, err := as.manager.SpawnAgent(ctx, req); err != nil {
				as.logger.WithError(err).Error("Failed to spawn agent during scale up")
				return err
			}

			// Small delay between spawns to avoid overload
			time.Sleep(time.Second)
		}
	} else if decision.Action == "scale_down" {
		// Get agents managed by autoscaler
		autoscalerAgents := as.manager.ListAgents("autoscaler")

		// Delete excess agents
		deleteCount := -delta
		if deleteCount > len(autoscalerAgents) {
			deleteCount = len(autoscalerAgents)
		}

		for i := 0; i < deleteCount; i++ {
			if err := as.manager.DeleteAgent(ctx, autoscalerAgents[i].AgentID); err != nil {
				as.logger.WithError(err).Error("Failed to delete agent during scale down")
			}

			// Small delay between deletions
			time.Sleep(time.Second)
		}
	}

	as.lastScaleTime = time.Now()
	return nil
}

// ensureMinimumAgents ensures minimum agents are always running
func (as *AutoScaler) ensureMinimumAgents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-as.stopChan:
			return
		case <-time.After(5 * time.Minute):
			stats := as.manager.GetStats()
			totalAgents := stats["total_agents"].(int)

			if totalAgents < as.config.MinAgents {
				needed := as.config.MinAgents - totalAgents
				as.logger.WithField("needed", needed).Info("Spawning minimum agents")

				for i := 0; i < needed; i++ {
					req := &AgentSpawnRequest{
						UserID:        "autoscaler",
						Cores:         4,
						Memory:        8192,
						MaxConcurrent: 100,
						Timeout:       300,
					}
					as.manager.SpawnAgent(ctx, req)
					time.Sleep(time.Second)
				}
			}
		}
	}
}

// queryMetric queries a single metric from Prometheus
func (as *AutoScaler) queryMetric(ctx context.Context, query string) (float64, error) {
	result, warnings, err := as.prometheusAPI.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	if len(warnings) > 0 {
		as.logger.WithField("warnings", warnings).Warn("Prometheus query warnings")
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("empty result for query: %s", query)
	}

	return float64(vector[0].Value), nil
}

// GetConfig returns current autoscaler configuration
func (as *AutoScaler) GetConfig() *AutoScalerConfig {
	return as.config
}

// UpdateConfig updates autoscaler configuration (safe for concurrent use)
func (as *AutoScaler) UpdateConfig(config *AutoScalerConfig) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	as.config = config
	as.logger.WithField("config", config).Info("Autoscaler configuration updated")
}

func main() {
	// Configuration
	prometheusURL := getEnv("PROMETHEUS_URL", "http://localhost:9090")
	hypercoreAddr := getEnv("HYPERCORE_ADDR", "localhost:8000")
	registryURL := getEnv("REGISTRY_URL", "registry.vistara.dev")

	// Create agent manager
	manager := NewAgentManager(hypercoreAddr, registryURL, 50)

	// Create autoscaler with production config for 10k users
	config := DefaultAutoScalerConfig()
	autoscaler, err := NewAutoScaler(manager, prometheusURL, config)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create autoscaler")
	}

	// Start autoscaler
	ctx := context.Background()
	autoscaler.Start(ctx)

	logrus.Info("Autoscaler started successfully")

	// Block forever
	select {}
}
