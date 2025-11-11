package policy

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/hashicorp/serf/serf"
	"github.com/sirupsen/logrus"
	pb "vistara-node/pkg/proto/cluster"
)

// Engine evaluates policies and selects nodes for workload placement
type Engine struct {
	logger        *logrus.Logger
	currentPolicy *Policy
	policyMutex   sync.RWMutex

	// Metrics
	evaluations      int64
	violations       int64
	metricsMutex     sync.RWMutex
}

// NewEngine creates a new policy engine
func NewEngine(logger *logrus.Logger) *Engine {
	return &Engine{
		logger:        logger,
		currentPolicy: DefaultPolicy(),
		evaluations:   0,
		violations:    0,
	}
}

// LoadPolicy loads a policy from a file path
func (e *Engine) LoadPolicy(policyPath string) error {
	if policyPath == "" {
		e.logger.Info("no policy file specified, using default policy")
		e.SetPolicy(DefaultPolicy())
		return nil
	}

	data, err := os.ReadFile(policyPath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	policy, err := LoadPolicyFromJSON(data)
	if err != nil {
		return err
	}

	if err := policy.Validate(); err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	e.SetPolicy(policy)
	e.logger.WithFields(logrus.Fields{
		"name": policy.Name,
		"mode": policy.Mode,
	}).Info("loaded policy")

	return nil
}

// SetPolicy sets the current policy
func (e *Engine) SetPolicy(policy *Policy) {
	e.policyMutex.Lock()
	defer e.policyMutex.Unlock()
	e.currentPolicy = policy
}

// GetPolicy returns the current policy
func (e *Engine) GetPolicy() *Policy {
	e.policyMutex.RLock()
	defer e.policyMutex.RUnlock()
	return e.currentPolicy
}

// CanSpawn checks if a spawn request is allowed by the current policy
func (e *Engine) CanSpawn(req *pb.VmSpawnRequest) (bool, string) {
	e.incrementEvaluations()

	policy := e.GetPolicy()

	// In permissive mode, always allow
	if policy.Mode == "permissive" {
		return true, ""
	}

	// In enforce mode, check constraints
	// For now, always allow spawn requests - node selection happens in SelectNodes
	return true, ""
}

// SelectNodes selects the best nodes for a workload based on the current policy
func (e *Engine) SelectNodes(req *pb.VmSpawnRequest, members []serf.Member, stateMap map[string]*pb.NodeStateResponse) ([]string, error) {
	e.incrementEvaluations()

	policy := e.GetPolicy()

	// Build candidate list with scores
	type candidate struct {
		name     string
		score    float64
		metadata *pb.BeaconMetadata
	}

	var candidates []candidate

	for _, member := range members {
		// Skip if not alive
		if member.Status != serf.StatusAlive {
			continue
		}

		// Get state for this node
		state, hasState := stateMap[member.Name]
		if !hasState || state.Beacon == nil {
			e.logger.WithField("node", member.Name).Debug("node has no beacon metadata, using permissive evaluation")

			// In permissive mode, include nodes without beacon data
			if policy.Mode == "permissive" {
				candidates = append(candidates, candidate{
					name:     member.Name,
					score:    0.5, // Neutral score
					metadata: nil,
				})
			}
			continue
		}

		beacon := state.Beacon

		// Check hard constraints
		if !e.meetsConstraints(policy, beacon) {
			e.logger.WithFields(logrus.Fields{
				"node":       member.Name,
				"latency":    beacon.LatencyMs,
				"price":      beacon.PricePerGb,
				"reputation": beacon.ReputationScore,
			}).Debug("node does not meet policy constraints")

			e.incrementViolations()
			continue
		}

		// Calculate score
		score := e.calculateScore(policy, beacon)

		candidates = append(candidates, candidate{
			name:     member.Name,
			score:    score,
			metadata: beacon,
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no nodes match policy constraints")
	}

	// Sort candidates by score (descending)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Log top candidates
	e.logger.WithFields(logrus.Fields{
		"policy":     policy.Name,
		"candidates": len(candidates),
		"top_node":   candidates[0].name,
		"top_score":  candidates[0].score,
	}).Info("policy evaluation completed")

	// Return node names in score order
	nodeNames := make([]string, len(candidates))
	for i, c := range candidates {
		nodeNames[i] = c.name
	}

	return nodeNames, nil
}

// meetsConstraints checks if a node meets the hard constraints
func (e *Engine) meetsConstraints(policy *Policy, beacon *pb.BeaconMetadata) bool {
	rules := policy.Rules

	// Check latency
	if rules.MaxLatencyMs > 0 && beacon.LatencyMs > rules.MaxLatencyMs {
		return false
	}

	// Check price
	if rules.MaxPricePerGB > 0 && beacon.PricePerGb > rules.MaxPricePerGB {
		return false
	}

	// Check reputation
	if rules.MinReputationScore > 0 {
		repScore, err := strconv.ParseFloat(beacon.ReputationScore, 64)
		if err != nil || repScore < rules.MinReputationScore {
			return false
		}
	}

	// Check queue depth
	if rules.MaxQueueDepth > 0 && beacon.QueueDepth > rules.MaxQueueDepth {
		return false
	}

	// Check packet loss
	if rules.MaxPacketLoss > 0 && beacon.PacketLoss > rules.MaxPacketLoss {
		return false
	}

	// Check jitter
	if rules.MaxJitterMs > 0 && beacon.JitterMs > rules.MaxJitterMs {
		return false
	}

	// Check required capabilities
	if len(rules.RequiredCapabilities) > 0 {
		capMap := make(map[string]bool)
		for _, cap := range beacon.NodeCapabilities {
			capMap[cap] = true
		}

		for _, required := range rules.RequiredCapabilities {
			if !capMap[required] {
				return false
			}
		}
	}

	return true
}

// calculateScore computes a score for a node based on policy weights
func (e *Engine) calculateScore(policy *Policy, beacon *pb.BeaconMetadata) float64 {
	weights := policy.Scoring
	score := 0.0

	// Latency score (lower is better) - normalize to 0-1
	if weights.LatencyWeight > 0 {
		// Assume 200ms is "bad" latency, 0ms is perfect
		latencyScore := 1.0 - (beacon.LatencyMs / 200.0)
		if latencyScore < 0 {
			latencyScore = 0
		}
		score += weights.LatencyWeight * latencyScore
	}

	// Price score (lower is better) - normalize to 0-1
	if weights.PriceWeight > 0 {
		// Assume $1/GB is "expensive", $0 is free
		priceScore := 1.0 - (beacon.PricePerGb / 1.0)
		if priceScore < 0 {
			priceScore = 0
		}
		score += weights.PriceWeight * priceScore
	}

	// Reputation score (higher is better) - already 0-1
	if weights.ReputationWeight > 0 {
		repScore, err := strconv.ParseFloat(beacon.ReputationScore, 64)
		if err == nil {
			score += weights.ReputationWeight * repScore
		}
	}

	// Queue depth score (lower is better) - normalize to 0-1
	if weights.QueueWeight > 0 {
		// Assume queue depth of 100 is "bad", 0 is perfect
		queueScore := 1.0 - (float64(beacon.QueueDepth) / 100.0)
		if queueScore < 0 {
			queueScore = 0
		}
		score += weights.QueueWeight * queueScore
	}

	return score
}

// GetMetrics returns policy engine metrics
func (e *Engine) GetMetrics() (evaluations int64, violations int64) {
	e.metricsMutex.RLock()
	defer e.metricsMutex.RUnlock()
	return e.evaluations, e.violations
}

func (e *Engine) incrementEvaluations() {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	e.evaluations++
}

func (e *Engine) incrementViolations() {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	e.violations++
}
