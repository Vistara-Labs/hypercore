package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentManagerCreation(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 10)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.maxAgentsPerUser != 10 {
		t.Errorf("Expected maxAgentsPerUser to be 10, got %d", manager.maxAgentsPerUser)
	}

	if len(manager.agents) != 0 {
		t.Errorf("Expected agents map to be empty, got %d", len(manager.agents))
	}
}

func TestValidateRequest(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 10)

	tests := []struct {
		name    string
		req     *AgentSpawnRequest
		wantErr bool
	}{
		{
			name: "Valid request",
			req: &AgentSpawnRequest{
				UserID: "user-123",
				Cores:  4,
				Memory: 8192,
			},
			wantErr: false,
		},
		{
			name: "Missing user ID",
			req: &AgentSpawnRequest{
				Cores:  4,
				Memory: 8192,
			},
			wantErr: true,
		},
		{
			name: "Invalid cores",
			req: &AgentSpawnRequest{
				UserID: "user-123",
				Cores:  100,
				Memory: 8192,
			},
			wantErr: true,
		},
		{
			name: "Invalid memory",
			req: &AgentSpawnRequest{
				UserID: "user-123",
				Cores:  4,
				Memory: 100000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckUserLimit(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 3)

	// Add 3 agents for user-123
	for i := 0; i < 3; i++ {
		agent := &AgentInfo{
			AgentID:   string(rune(i)),
			UserID:    "user-123",
			CreatedAt: time.Now(),
		}
		manager.agents[agent.AgentID] = agent
	}

	// Should fail for user-123 (at limit)
	err := manager.checkUserLimit("user-123")
	if err == nil {
		t.Error("Expected error for user at limit")
	}

	// Should succeed for user-456 (no agents)
	err = manager.checkUserLimit("user-456")
	if err != nil {
		t.Errorf("Expected no error for new user, got: %v", err)
	}
}

func TestGetAgent(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 10)

	// Add test agent
	testAgent := &AgentInfo{
		AgentID:   "agent-123",
		UserID:    "user-123",
		URL:       "agent-123.example.com",
		CreatedAt: time.Now(),
	}
	manager.agents["agent-123"] = testAgent

	// Test getting existing agent
	agent, err := manager.GetAgent("agent-123")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if agent.AgentID != "agent-123" {
		t.Errorf("Expected agent ID agent-123, got: %s", agent.AgentID)
	}

	// Test getting non-existent agent
	_, err = manager.GetAgent("invalid-agent")
	if err == nil {
		t.Error("Expected error for non-existent agent")
	}
}

func TestListAgents(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 10)

	// Add agents for multiple users
	agents := []*AgentInfo{
		{AgentID: "agent-1", UserID: "user-123"},
		{AgentID: "agent-2", UserID: "user-123"},
		{AgentID: "agent-3", UserID: "user-456"},
	}

	for _, agent := range agents {
		manager.agents[agent.AgentID] = agent
	}

	// List agents for user-123
	userAgents := manager.ListAgents("user-123")
	if len(userAgents) != 2 {
		t.Errorf("Expected 2 agents for user-123, got %d", len(userAgents))
	}

	// List agents for user-456
	userAgents = manager.ListAgents("user-456")
	if len(userAgents) != 1 {
		t.Errorf("Expected 1 agent for user-456, got %d", len(userAgents))
	}

	// List agents for non-existent user
	userAgents = manager.ListAgents("user-999")
	if len(userAgents) != 0 {
		t.Errorf("Expected 0 agents for user-999, got %d", len(userAgents))
	}
}

func TestGetStats(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 10)

	// Add test agents
	manager.agents["agent-1"] = &AgentInfo{UserID: "user-123"}
	manager.agents["agent-2"] = &AgentInfo{UserID: "user-123"}
	manager.agents["agent-3"] = &AgentInfo{UserID: "user-456"}

	stats := manager.GetStats()

	totalAgents, ok := stats["total_agents"].(int)
	if !ok || totalAgents != 3 {
		t.Errorf("Expected total_agents to be 3, got %v", stats["total_agents"])
	}

	users, ok := stats["users"].(int)
	if !ok || users != 2 {
		t.Errorf("Expected users to be 2, got %v", stats["users"])
	}
}

func TestHTTPHandlers(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 10)

	// Add a test agent
	manager.agents["agent-123"] = &AgentInfo{
		AgentID:   "agent-123",
		UserID:    "user-123",
		URL:       "agent-123.example.com",
		CreatedAt: time.Now(),
	}

	t.Run("GET /v1/agents/get", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/agents/get?agent_id=agent-123", nil)
		w := httptest.NewRecorder()

		manager.handleGet(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var agent AgentInfo
		json.NewDecoder(w.Body).Decode(&agent)

		if agent.AgentID != "agent-123" {
			t.Errorf("Expected agent ID agent-123, got %s", agent.AgentID)
		}
	})

	t.Run("GET /v1/agents/list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/agents/list?user_id=user-123", nil)
		w := httptest.NewRecorder()

		manager.handleList(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result map[string]interface{}
		json.NewDecoder(w.Body).Decode(&result)

		if result["user_id"] != "user-123" {
			t.Errorf("Expected user_id user-123, got %v", result["user_id"])
		}
	})

	t.Run("GET /v1/agents/stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/agents/stats", nil)
		w := httptest.NewRecorder()

		manager.handleStats(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var stats map[string]interface{}
		json.NewDecoder(w.Body).Decode(&stats)

		if stats["total_agents"] != float64(1) {
			t.Errorf("Expected total_agents 1, got %v", stats["total_agents"])
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewAgentManager("localhost:8000", "registry.example.com", 100)

	// Simulate concurrent agent additions
	done := make(chan bool)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			agent := &AgentInfo{
				AgentID:   string(rune(id)),
				UserID:    "concurrent-user",
				CreatedAt: time.Now(),
			}

			manager.agentsMutex.Lock()
			manager.agents[agent.AgentID] = agent
			manager.agentsMutex.Unlock()

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check no errors
	select {
	case err := <-errors:
		t.Errorf("Unexpected error: %v", err)
	default:
		// No errors, good
	}

	// Verify all agents were added
	if len(manager.agents) != 10 {
		t.Errorf("Expected 10 agents, got %d", len(manager.agents))
	}
}

// Mock HTTP server for testing hypercore calls
func TestCallHypercoreSpawn(t *testing.T) {
	// Create mock hypercore server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/spawn" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		response := map[string]string{
			"id":  "test-agent-123",
			"url": "test-agent-123.example.com",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	manager := NewAgentManager(server.URL[7:], "registry.example.com", 10) // Remove http://

	payload := map[string]interface{}{
		"imageRef": "registry.example.com/claude-agent:latest",
		"cores":    4,
		"memory":   8192,
	}

	ctx := context.Background()
	agentID, url, err := manager.callHypercoreSpawn(ctx, payload)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if agentID != "test-agent-123" {
		t.Errorf("Expected agent ID test-agent-123, got: %s", agentID)
	}

	if url != "test-agent-123.example.com" {
		t.Errorf("Expected URL test-agent-123.example.com, got: %s", url)
	}
}
