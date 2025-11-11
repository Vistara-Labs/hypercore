package policy

import (
	"encoding/json"
	"fmt"
)

// Policy defines the structure for workload placement policies
type Policy struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Mode        string       `json:"mode"` // "enforce" or "permissive"
	Rules       PolicyRules  `json:"rules"`
	Scoring     ScoreWeights `json:"scoring,omitempty"`
}

// PolicyRules defines constraints and preferences for node selection
type PolicyRules struct {
	// Hard constraints (must match)
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	MaxLatencyMs         float64  `json:"max_latency_ms,omitempty"`
	MaxPricePerGB        float64  `json:"max_price_per_gb,omitempty"`
	MinReputationScore   float64  `json:"min_reputation_score,omitempty"`
	MaxQueueDepth        uint32   `json:"max_queue_depth,omitempty"`
	MaxPacketLoss        float64  `json:"max_packet_loss,omitempty"`
	MaxJitterMs          float64  `json:"max_jitter_ms,omitempty"`

	// Soft preferences (ranked by score)
	PreferLowLatency bool `json:"prefer_low_latency,omitempty"`
	PreferLowPrice   bool `json:"prefer_low_price,omitempty"`
	PreferHighReputation bool `json:"prefer_high_reputation,omitempty"`
}

// ScoreWeights defines how to weight different metrics when ranking nodes
type ScoreWeights struct {
	LatencyWeight    float64 `json:"latency_weight"`    // Higher = prioritize low latency
	PriceWeight      float64 `json:"price_weight"`      // Higher = prioritize low price
	ReputationWeight float64 `json:"reputation_weight"` // Higher = prioritize high reputation
	QueueWeight      float64 `json:"queue_weight"`      // Higher = prioritize low queue depth
}

// DefaultPolicy returns a permissive default policy
func DefaultPolicy() *Policy {
	return &Policy{
		Name:        "default",
		Description: "Default permissive policy - accepts any node",
		Mode:        "permissive",
		Rules: PolicyRules{
			RequiredCapabilities: []string{},
			MaxLatencyMs:         0, // 0 = no limit
			MaxPricePerGB:        0,
			MinReputationScore:   0,
			MaxQueueDepth:        0,
			MaxPacketLoss:        100.0,
			MaxJitterMs:          0,
		},
		Scoring: ScoreWeights{
			LatencyWeight:    0.25,
			PriceWeight:      0.25,
			ReputationWeight: 0.25,
			QueueWeight:      0.25,
		},
	}
}

// LoadPolicyFromJSON parses a policy from JSON bytes
func LoadPolicyFromJSON(data []byte) (*Policy, error) {
	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy JSON: %w", err)
	}

	// Validate mode
	if policy.Mode != "enforce" && policy.Mode != "permissive" {
		return nil, fmt.Errorf("invalid policy mode: %s (must be 'enforce' or 'permissive')", policy.Mode)
	}

	// Normalize score weights
	if policy.Scoring.LatencyWeight == 0 && policy.Scoring.PriceWeight == 0 &&
		policy.Scoring.ReputationWeight == 0 && policy.Scoring.QueueWeight == 0 {
		// No weights specified, use defaults
		policy.Scoring = DefaultPolicy().Scoring
	}

	return &policy, nil
}

// ToJSON serializes the policy to JSON
func (p *Policy) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// Validate checks if the policy is valid
func (p *Policy) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("policy name cannot be empty")
	}

	if p.Mode != "enforce" && p.Mode != "permissive" {
		return fmt.Errorf("invalid mode: %s", p.Mode)
	}

	// Validate constraints make sense
	if p.Rules.MaxLatencyMs < 0 {
		return fmt.Errorf("max_latency_ms cannot be negative")
	}

	if p.Rules.MaxPricePerGB < 0 {
		return fmt.Errorf("max_price_per_gb cannot be negative")
	}

	if p.Rules.MinReputationScore < 0 || p.Rules.MinReputationScore > 1 {
		return fmt.Errorf("min_reputation_score must be between 0 and 1")
	}

	if p.Rules.MaxPacketLoss < 0 || p.Rules.MaxPacketLoss > 100 {
		return fmt.Errorf("max_packet_loss must be between 0 and 100")
	}

	return nil
}
