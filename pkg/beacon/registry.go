package beacon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	pb "vistara-node/pkg/proto/cluster"
)

// Registry maintains a registry of known nodes and their beacon metadata
type Registry struct {
	logger  *logrus.Logger
	nodes   map[string]*NodeRecord
	mutex   sync.RWMutex
}

// NodeRecord represents a registered node with its metadata
type NodeRecord struct {
	NodeID       string
	Metadata     *pb.BeaconMetadata
	LastSeen     time.Time
	Verified     bool
}

// NewRegistry creates a new beacon registry
func NewRegistry(logger *logrus.Logger) *Registry {
	return &Registry{
		logger: logger,
		nodes:  make(map[string]*NodeRecord),
	}
}

// Register registers or updates a node in the registry
func (r *Registry) Register(ctx context.Context, nodeID string, metadata *pb.BeaconMetadata) error {
	if nodeID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	record, exists := r.nodes[nodeID]
	if exists {
		// Update existing record
		record.Metadata = metadata
		record.LastSeen = time.Now()
		r.logger.WithField("node_id", nodeID).Debug("updated node registration")
	} else {
		// Create new record
		r.nodes[nodeID] = &NodeRecord{
			NodeID:   nodeID,
			Metadata: metadata,
			LastSeen: time.Now(),
			Verified: false, // Will be verified after attestation
		}
		r.logger.WithField("node_id", nodeID).Info("registered new node")
	}

	return nil
}

// Get retrieves a node record from the registry
func (r *Registry) Get(nodeID string) (*NodeRecord, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found in registry", nodeID)
	}

	return record, nil
}

// List returns all registered nodes
func (r *Registry) List() []*NodeRecord {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	records := make([]*NodeRecord, 0, len(r.nodes))
	for _, record := range r.nodes {
		records = append(records, record)
	}

	return records
}

// Remove removes a node from the registry
func (r *Registry) Remove(nodeID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found in registry", nodeID)
	}

	delete(r.nodes, nodeID)
	r.logger.WithField("node_id", nodeID).Info("removed node from registry")

	return nil
}

// Cleanup removes stale nodes that haven't been seen for a specified duration
func (r *Registry) Cleanup(maxAge time.Duration) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	removed := 0
	now := time.Now()

	for nodeID, record := range r.nodes {
		if now.Sub(record.LastSeen) > maxAge {
			delete(r.nodes, nodeID)
			removed++
			r.logger.WithFields(logrus.Fields{
				"node_id":   nodeID,
				"last_seen": record.LastSeen,
			}).Info("removed stale node from registry")
		}
	}

	return removed
}

// Count returns the number of registered nodes
func (r *Registry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.nodes)
}

// MarkVerified marks a node as verified after successful attestation
func (r *Registry) MarkVerified(nodeID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found in registry", nodeID)
	}

	record.Verified = true
	r.logger.WithField("node_id", nodeID).Info("marked node as verified")

	return nil
}

// GetByMetrics returns nodes that match the specified metric criteria
func (r *Registry) GetByMetrics(maxLatency float64, minReputation string) []*NodeRecord {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var matches []*NodeRecord

	for _, record := range r.nodes {
		if record.Metadata == nil {
			continue
		}

		// Filter by latency
		if maxLatency > 0 && record.Metadata.LatencyMs > maxLatency {
			continue
		}

		// Filter by reputation (simple string comparison for now)
		if minReputation != "" && record.Metadata.ReputationScore < minReputation {
			continue
		}

		matches = append(matches, record)
	}

	return matches
}
