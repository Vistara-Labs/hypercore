package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Prometheus metrics
var (
	agentSpawnTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypercore_agent_spawn_total",
			Help: "Total number of agent spawn requests",
		},
		[]string{"status"},
	)

	agentActiveGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hypercore_agent_active_count",
			Help: "Number of active agent containers",
		},
	)

	agentSpawnDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "hypercore_agent_spawn_duration_seconds",
			Help:    "Duration of agent spawn operations",
			Buckets: prometheus.DefBuckets,
		},
	)
)

// AgentSpawnRequest represents a request to spawn a Claude agent
type AgentSpawnRequest struct {
	UserID          string            `json:"user_id"`
	AnthropicAPIKey string            `json:"anthropic_api_key,omitempty"`
	Cores           int               `json:"cores,omitempty"`
	Memory          int               `json:"memory,omitempty"` // MB
	GPUQuota        int               `json:"gpu_quota,omitempty"` // MB VRAM
	MaxConcurrent   int               `json:"max_concurrent,omitempty"`
	Timeout         int               `json:"timeout,omitempty"` // seconds
	Tags            map[string]string `json:"tags,omitempty"`
	SystemPrompt    string            `json:"system_prompt,omitempty"`
}

// AgentSpawnResponse represents the response from spawning an agent
type AgentSpawnResponse struct {
	AgentID     string    `json:"agent_id"`
	URL         string    `json:"url"`
	MetricsURL  string    `json:"metrics_url"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UserID      string    `json:"user_id"`
	Cores       int       `json:"cores"`
	Memory      int       `json:"memory"`
}

// AgentInfo stores information about a running agent
type AgentInfo struct {
	AgentID     string
	UserID      string
	URL         string
	MetricsURL  string
	CreatedAt   time.Time
	LastActive  time.Time
	Status      string
}

// AgentManager manages Claude agent lifecycle in hypercore
type AgentManager struct {
	agents          map[string]*AgentInfo
	agentsMutex     sync.RWMutex
	hypercoreAddr   string
	registryURL     string
	logger          *logrus.Logger
	maxAgentsPerUser int
}

// NewAgentManager creates a new agent manager
func NewAgentManager(hypercoreAddr, registryURL string, maxAgentsPerUser int) *AgentManager {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	return &AgentManager{
		agents:           make(map[string]*AgentInfo),
		hypercoreAddr:    hypercoreAddr,
		registryURL:      registryURL,
		logger:           logger,
		maxAgentsPerUser: maxAgentsPerUser,
	}
}

// SpawnAgent spawns a new Claude agent container via hypercore
func (am *AgentManager) SpawnAgent(ctx context.Context, req *AgentSpawnRequest) (*AgentSpawnResponse, error) {
	start := time.Now()

	// Validate request
	if err := am.validateRequest(req); err != nil {
		agentSpawnTotal.WithLabelValues("validation_error").Inc()
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Check user's agent limit
	if err := am.checkUserLimit(req.UserID); err != nil {
		agentSpawnTotal.WithLabelValues("limit_exceeded").Inc()
		return nil, err
	}

	am.logger.WithFields(logrus.Fields{
		"user_id": req.UserID,
		"cores":   req.Cores,
		"memory":  req.Memory,
	}).Info("Spawning new Claude agent")

	// Default values
	if req.Cores == 0 {
		req.Cores = 4
	}
	if req.Memory == 0 {
		req.Memory = 8192 // 8GB
	}
	if req.MaxConcurrent == 0 {
		req.MaxConcurrent = 100
	}
	if req.Timeout == 0 {
		req.Timeout = 300
	}

	// Build hypercore spawn command
	spawnPayload := map[string]interface{}{
		"imageRef": fmt.Sprintf("%s/claude-agent:latest", am.registryURL),
		"cores":    req.Cores,
		"memory":   req.Memory,
		"ports":    []string{"443:8080", "9090:9090"},
		"env": map[string]string{
			"ANTHROPIC_API_KEY":      req.AnthropicAPIKey,
			"MAX_CONCURRENT_REQUESTS": fmt.Sprintf("%d", req.MaxConcurrent),
			"REQUEST_TIMEOUT":        fmt.Sprintf("%d", req.Timeout),
			"LOG_LEVEL":              "info",
		},
	}

	// Add GPU quota if specified
	if req.GPUQuota > 0 {
		spawnPayload["gpu_quota"] = req.GPUQuota
	}

	// Call hypercore spawn API
	agentID, url, err := am.callHypercoreSpawn(ctx, spawnPayload)
	if err != nil {
		agentSpawnTotal.WithLabelValues("spawn_error").Inc()
		return nil, fmt.Errorf("hypercore spawn failed: %w", err)
	}

	// Store agent info
	agentInfo := &AgentInfo{
		AgentID:    agentID,
		UserID:     req.UserID,
		URL:        url,
		MetricsURL: fmt.Sprintf("https://%s/metrics", url),
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     "running",
	}

	am.agentsMutex.Lock()
	am.agents[agentID] = agentInfo
	am.agentsMutex.Unlock()

	agentActiveGauge.Inc()
	agentSpawnTotal.WithLabelValues("success").Inc()
	agentSpawnDuration.Observe(time.Since(start).Seconds())

	am.logger.WithFields(logrus.Fields{
		"agent_id":   agentID,
		"user_id":    req.UserID,
		"url":        url,
		"duration_ms": time.Since(start).Milliseconds(),
	}).Info("Agent spawned successfully")

	return &AgentSpawnResponse{
		AgentID:    agentID,
		URL:        url,
		MetricsURL: agentInfo.MetricsURL,
		Status:     "running",
		CreatedAt:  agentInfo.CreatedAt,
		UserID:     req.UserID,
		Cores:      req.Cores,
		Memory:     req.Memory,
	}, nil
}

// DeleteAgent terminates a Claude agent
func (am *AgentManager) DeleteAgent(ctx context.Context, agentID string) error {
	am.agentsMutex.Lock()
	defer am.agentsMutex.Unlock()

	agent, exists := am.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	am.logger.WithField("agent_id", agentID).Info("Deleting agent")

	// Call hypercore to delete the VM
	if err := am.callHypercoreDelete(ctx, agentID); err != nil {
		return fmt.Errorf("hypercore delete failed: %w", err)
	}

	delete(am.agents, agentID)
	agentActiveGauge.Dec()

	am.logger.WithFields(logrus.Fields{
		"agent_id": agentID,
		"user_id":  agent.UserID,
		"uptime":   time.Since(agent.CreatedAt).String(),
	}).Info("Agent deleted successfully")

	return nil
}

// GetAgent retrieves agent information
func (am *AgentManager) GetAgent(agentID string) (*AgentInfo, error) {
	am.agentsMutex.RLock()
	defer am.agentsMutex.RUnlock()

	agent, exists := am.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	return agent, nil
}

// ListAgents lists all agents for a user
func (am *AgentManager) ListAgents(userID string) []*AgentInfo {
	am.agentsMutex.RLock()
	defer am.agentsMutex.RUnlock()

	var userAgents []*AgentInfo
	for _, agent := range am.agents {
		if agent.UserID == userID {
			userAgents = append(userAgents, agent)
		}
	}

	return userAgents
}

// GetStats returns agent statistics
func (am *AgentManager) GetStats() map[string]interface{} {
	am.agentsMutex.RLock()
	defer am.agentsMutex.RUnlock()

	userCounts := make(map[string]int)
	for _, agent := range am.agents {
		userCounts[agent.UserID]++
	}

	return map[string]interface{}{
		"total_agents":    len(am.agents),
		"users":           len(userCounts),
		"agents_per_user": userCounts,
	}
}

// validateRequest validates spawn request
func (am *AgentManager) validateRequest(req *AgentSpawnRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.Cores < 0 || req.Cores > 32 {
		return fmt.Errorf("cores must be between 0 and 32")
	}
	if req.Memory < 0 || req.Memory > 65536 {
		return fmt.Errorf("memory must be between 0 and 65536 MB")
	}
	return nil
}

// checkUserLimit checks if user has reached agent limit
func (am *AgentManager) checkUserLimit(userID string) error {
	am.agentsMutex.RLock()
	defer am.agentsMutex.RUnlock()

	count := 0
	for _, agent := range am.agents {
		if agent.UserID == userID {
			count++
		}
	}

	if count >= am.maxAgentsPerUser {
		return fmt.Errorf("user %s has reached maximum agents limit: %d", userID, am.maxAgentsPerUser)
	}

	return nil
}

// callHypercoreSpawn calls hypercore API to spawn container
func (am *AgentManager) callHypercoreSpawn(ctx context.Context, payload map[string]interface{}) (string, string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	url := fmt.Sprintf("http://%s/spawn", am.hypercoreAddr)
	req, err := http.NewRequestWithContext(ctx, "POST", url, io.NopCloser(io.Reader(payloadBytes)))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("hypercore spawn failed: %s", string(body))
	}

	var result struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.ID, result.URL, nil
}

// callHypercoreDelete calls hypercore API to delete container
func (am *AgentManager) callHypercoreDelete(ctx context.Context, agentID string) error {
	url := fmt.Sprintf("http://%s/delete/%s", am.hypercoreAddr, agentID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hypercore delete failed: %s", string(body))
	}

	return nil
}

// HTTP Handlers
func (am *AgentManager) handleSpawn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentSpawnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := am.SpawnAgent(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (am *AgentManager) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	if err := am.DeleteAgent(r.Context(), agentID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "agent_id": agentID})
}

func (am *AgentManager) handleGet(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	agent, err := am.GetAgent(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (am *AgentManager) handleList(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	agents := am.ListAgents(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"count":   len(agents),
		"agents":  agents,
	})
}

func (am *AgentManager) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := am.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Configuration from environment
	hypercoreAddr := getEnv("HYPERCORE_ADDR", "localhost:8000")
	registryURL := getEnv("REGISTRY_URL", "registry.vistara.dev")
	maxAgentsPerUser := getEnvInt("MAX_AGENTS_PER_USER", 10)
	listenAddr := getEnv("LISTEN_ADDR", ":8080")

	// Create agent manager
	manager := NewAgentManager(hypercoreAddr, registryURL, maxAgentsPerUser)

	// Setup HTTP routes
	http.HandleFunc("/v1/agents/spawn", manager.handleSpawn)
	http.HandleFunc("/v1/agents/delete", manager.handleDelete)
	http.HandleFunc("/v1/agents/get", manager.handleGet)
	http.HandleFunc("/v1/agents/list", manager.handleList)
	http.HandleFunc("/v1/agents/stats", manager.handleStats)
	http.Handle("/metrics", promhttp.Handler())

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	manager.logger.WithField("addr", listenAddr).Info("Starting agent manager server")
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		manager.logger.WithError(err).Fatal("Server failed")
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}
