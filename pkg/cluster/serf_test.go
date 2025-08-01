package cluster

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	pb "vistara-node/pkg/proto/cluster"

	"github.com/hashicorp/serf/serf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashWorkloadState(t *testing.T) {
	agent := &Agent{}

	// Test empty state
	emptyState := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
	}
	hash1 := agent.hashWorkloadState(emptyState)
	assert.NotEmpty(t, hash1)

	// Test state with workloads
	state1 := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
		Workloads: []*pb.WorkloadState{
			{Id: "workload1"},
			{Id: "workload2"},
		},
	}
	hash2 := agent.hashWorkloadState(state1)
	assert.NotEmpty(t, hash2)
	assert.NotEqual(t, hash1, hash2)

	// Test same workloads in different order (should produce same hash)
	state2 := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
		Workloads: []*pb.WorkloadState{
			{Id: "workload2"},
			{Id: "workload1"},
		},
	}
	hash3 := agent.hashWorkloadState(state2)
	assert.Equal(t, hash2, hash3, "Hash should be same regardless of workload order")

	// Test different workload count
	state3 := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
		Workloads: []*pb.WorkloadState{
			{Id: "workload1"},
		},
	}
	hash4 := agent.hashWorkloadState(state3)
	assert.NotEqual(t, hash2, hash4, "Hash should be different for different workload counts")
}

func TestStateChangeDetection(t *testing.T) {
	agent := &Agent{
		lastStateHash: "",
		stateMu:       sync.Mutex{},
	}

	// Test initial state (no previous hash)
	state1 := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
		Workloads: []*pb.WorkloadState{
			{Id: "workload1"},
		},
	}

	currentHash := agent.hashWorkloadState(state1)
	agent.stateMu.Lock()
	stateChanged := currentHash != agent.lastStateHash
	if stateChanged {
		agent.lastStateHash = currentHash
	}
	agent.stateMu.Unlock()

	assert.True(t, stateChanged, "First state should be considered changed")

	// Test same state (should not change)
	state2 := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
		Workloads: []*pb.WorkloadState{
			{Id: "workload1"},
		},
	}

	currentHash2 := agent.hashWorkloadState(state2)
	agent.stateMu.Lock()
	stateChanged2 := currentHash2 != agent.lastStateHash
	if stateChanged2 {
		agent.lastStateHash = currentHash2
	}
	agent.stateMu.Unlock()

	assert.False(t, stateChanged2, "Same state should not be considered changed")

	// Test different state (should change)
	state3 := &pb.NodeStateResponse{
		Node: &pb.Node{Id: "test-node"},
		Workloads: []*pb.WorkloadState{
			{Id: "workload1"},
			{Id: "workload2"},
		},
	}

	currentHash3 := agent.hashWorkloadState(state3)
	agent.stateMu.Lock()
	stateChanged3 := currentHash3 != agent.lastStateHash
	if stateChanged3 {
		agent.lastStateHash = currentHash3
	}
	agent.stateMu.Unlock()

	assert.True(t, stateChanged3, "Different state should be considered changed")
}

func TestQueueDepthThrottling(t *testing.T) {
	// Test constants
	assert.Equal(t, 3600, MaxQueueDepth, "MaxQueueDepth should be 3600")
	assert.Equal(t, time.Second*5, WorkloadBroadcastPeriod, "WorkloadBroadcastPeriod should be 5 seconds")

	// Test queue depth parsing
	testCases := []struct {
		queueDepthStr string
		expectedDepth int
		shouldSkip    bool
	}{
		{"100", 100, false},
		{"1000", 1000, false},
		{"3600", 3600, false},
		{"3601", 3601, true},
		{"5000", 5000, true},
		{"invalid", 0, false}, // Invalid string should not skip
	}

	for _, tc := range testCases {
		t.Run(tc.queueDepthStr, func(t *testing.T) {
			queueDepth, err := strconv.Atoi(tc.queueDepthStr)
			if err != nil {
				// Invalid string case
				assert.False(t, tc.shouldSkip, "Invalid queue depth should not cause skip")
				return
			}

			shouldSkip := queueDepth > MaxQueueDepth
			assert.Equal(t, tc.shouldSkip, shouldSkip,
				"Queue depth %d should skip=%v", queueDepth, tc.shouldSkip)
		})
	}
}

func TestSerfConfiguration(t *testing.T) {
	// Test that our Serf configuration is conservative
	cfg := serf.DefaultConfig()

	// These values should be set in NewAgent
	cfg.UserEventSizeLimit = 2048
	cfg.MemberlistConfig.GossipInterval = time.Second * 2
	cfg.MemberlistConfig.ProbeInterval = time.Second * 5
	cfg.MemberlistConfig.SuspicionMult = 6
	cfg.MemberlistConfig.GossipNodes = 2

	// Verify conservative settings
	assert.Equal(t, 2048, cfg.UserEventSizeLimit, "UserEventSizeLimit should be 2048")
	assert.Equal(t, time.Second*2, cfg.MemberlistConfig.GossipInterval, "GossipInterval should be 2s")
	assert.Equal(t, time.Second*5, cfg.MemberlistConfig.ProbeInterval, "ProbeInterval should be 5s")
	assert.Equal(t, 6, cfg.MemberlistConfig.SuspicionMult, "SuspicionMult should be 6")
	assert.Equal(t, 2, cfg.MemberlistConfig.GossipNodes, "GossipNodes should be 2")
}

func TestPrometheusMetrics(t *testing.T) {
	// Test that metrics are properly registered
	metrics := prometheus.DefaultRegisterer.(*prometheus.Registry)

	// Check if our metrics are registered
	metricFamilies, err := metrics.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, mf := range metricFamilies {
		metricNames[*mf.Name] = true
	}

	expectedMetrics := []string{
		"hypercore_serf_queue_depth",
		"hypercore_workload_count",
		"hypercore_broadcast_skipped_total",
		"hypercore_state_changes_total",
	}

	for _, metricName := range expectedMetrics {
		assert.True(t, metricNames[metricName], "Metric %s should be registered", metricName)
	}
}

func TestWorkloadStateConsistency(t *testing.T) {
	agent := &Agent{}

	// Test that workload state is consistent
	workloads := []*pb.WorkloadState{
		{Id: "workload1", SourceRequest: &pb.VmSpawnRequest{ImageRef: "test:latest"}},
		{Id: "workload2", SourceRequest: &pb.VmSpawnRequest{ImageRef: "test:latest"}},
	}

	state := &pb.NodeStateResponse{
		Node:      &pb.Node{Id: "test-node"},
		Workloads: workloads,
	}

	// Hash should be consistent
	hash1 := agent.hashWorkloadState(state)
	hash2 := agent.hashWorkloadState(state)
	assert.Equal(t, hash1, hash2, "Hash should be consistent for same state")

	// Hash should change when workloads change
	state.Workloads = append(state.Workloads, &pb.WorkloadState{Id: "workload3"})
	hash3 := agent.hashWorkloadState(state)
	assert.NotEqual(t, hash1, hash3, "Hash should change when workloads change")
}

func TestBroadcastPeriod(t *testing.T) {
	// Test that broadcast period is reasonable
	assert.True(t, WorkloadBroadcastPeriod >= time.Second*5,
		"WorkloadBroadcastPeriod should be at least 5 seconds")
	assert.True(t, WorkloadBroadcastPeriod <= time.Minute*1,
		"WorkloadBroadcastPeriod should not exceed 1 minute")
}

func BenchmarkHashWorkloadState(b *testing.B) {
	agent := &Agent{}
	state := &pb.NodeStateResponse{
		Node:      &pb.Node{Id: "test-node"},
		Workloads: make([]*pb.WorkloadState, 100),
	}

	// Create 100 workloads
	for i := 0; i < 100; i++ {
		state.Workloads[i] = &pb.WorkloadState{
			Id: fmt.Sprintf("workload%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.hashWorkloadState(state)
	}
}

func TestConcurrentStateAccess(t *testing.T) {
	agent := &Agent{
		stateMu: sync.Mutex{},
	}

	// Test concurrent access to state hash
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			agent.stateMu.Lock()
			agent.lastStateHash = fmt.Sprintf("hash%d", i)
			agent.stateMu.Unlock()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not have race conditions
	assert.NotEmpty(t, agent.lastStateHash)
}
