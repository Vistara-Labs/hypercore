package beacon

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	pb "vistara-node/pkg/proto/cluster"
)

// Client represents a beacon client that manages node attestation and metrics
type Client struct {
	logger        *logrus.Logger
	endpoint      string
	nodeID        string
	privateKey    ed25519.PrivateKey
	publicKey     ed25519.PublicKey
	metrics       *NodeMetrics
	metricsMutex  sync.RWMutex
	connected     bool
	connMutex     sync.RWMutex
}

// NodeMetrics holds current metrics for the node
type NodeMetrics struct {
	LatencyMs      float64
	JitterMs       float64
	PacketLoss     float64
	QueueDepth     uint32
	PricePerGB     float64
	ReputationScore string
	Capabilities   []string
	LastUpdate     time.Time
}

// AttestationResult contains the result of an attestation request
type AttestationResult struct {
	NodeID    string
	Signature []byte
	Timestamp int64
	Valid     bool
}

// NewClient creates a new beacon client
func NewClient(logger *logrus.Logger, endpoint string) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Generate ed25519 keypair for signing attestations
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	nodeID := hex.EncodeToString(publicKey[:8]) // Use first 8 bytes as node ID

	client := &Client{
		logger:     logger,
		endpoint:   endpoint,
		nodeID:     nodeID,
		privateKey: privateKey,
		publicKey:  publicKey,
		metrics: &NodeMetrics{
			LatencyMs:       0.0,
			JitterMs:        0.0,
			PacketLoss:      0.0,
			QueueDepth:      0,
			PricePerGB:      0.01, // Default price
			ReputationScore: "1.0",
			Capabilities:    []string{"container", "vm"},
			LastUpdate:      time.Now(),
		},
		connected: false,
	}

	// If endpoint is provided, attempt connection
	if endpoint != "" {
		if err := client.Connect(context.Background()); err != nil {
			logger.WithError(err).Warn("failed to connect to beacon endpoint, operating in standalone mode")
		}
	} else {
		logger.Info("no beacon endpoint provided, operating in standalone mode")
	}

	return client, nil
}

// Connect establishes connection to the beacon network
func (c *Client) Connect(ctx context.Context) error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.endpoint == "" {
		return fmt.Errorf("no beacon endpoint configured")
	}

	c.logger.WithField("endpoint", c.endpoint).Info("connecting to beacon network")

	// Attempt HTTP health check first
	if strings.HasPrefix(c.endpoint, "http://") || strings.HasPrefix(c.endpoint, "https://") {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get(c.endpoint + "/health")
		if err != nil {
			c.logger.WithError(err).Warn("beacon endpoint health check failed, operating in standalone mode")
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			c.logger.WithField("status", resp.StatusCode).Warn("beacon endpoint returned non-OK status, operating in standalone mode")
			return fmt.Errorf("beacon endpoint returned status %d", resp.StatusCode)
		}
		c.connected = true
		c.logger.Info("successfully connected to beacon network via HTTP")
		return nil
	}

	// Attempt TCP connection for other protocols
	conn, err := net.DialTimeout("tcp", c.endpoint, 5*time.Second)
	if err != nil {
		c.logger.WithError(err).Warn("beacon endpoint connection failed, operating in standalone mode")
		return err
	}
	conn.Close()

	c.connected = true
	c.logger.Info("successfully connected to beacon network")
	return nil
}

// IsConnected returns whether the client is connected to the beacon network
func (c *Client) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// Attest generates a cryptographic attestation for the given node ID
func (c *Client) Attest(ctx context.Context, nodeID string) (*AttestationResult, error) {
	// Create attestation message
	timestamp := time.Now().Unix()
	message := fmt.Sprintf("%s:%d", nodeID, timestamp)

	// Sign the message
	signature := ed25519.Sign(c.privateKey, []byte(message))

	result := &AttestationResult{
		NodeID:    c.nodeID,
		Signature: signature,
		Timestamp: timestamp,
		Valid:     true,
	}

	c.logger.WithFields(logrus.Fields{
		"node_id":   nodeID,
		"timestamp": timestamp,
	}).Debug("generated attestation")

	return result, nil
}

// UpdateMetrics updates the current node metrics
func (c *Client) UpdateMetrics(latency, jitter, packetLoss float64, queueDepth uint32) {
	c.metricsMutex.Lock()
	defer c.metricsMutex.Unlock()

	c.metrics.LatencyMs = latency
	c.metrics.JitterMs = jitter
	c.metrics.PacketLoss = packetLoss
	c.metrics.QueueDepth = queueDepth
	c.metrics.LastUpdate = time.Now()

	c.logger.WithFields(logrus.Fields{
		"latency_ms":   latency,
		"jitter_ms":    jitter,
		"packet_loss":  packetLoss,
		"queue_depth":  queueDepth,
	}).Debug("updated node metrics")
}

// SetPrice sets the price per GB for this node
func (c *Client) SetPrice(pricePerGB float64) {
	c.metricsMutex.Lock()
	defer c.metricsMutex.Unlock()
	c.metrics.PricePerGB = pricePerGB
}

// SetReputationScore sets the reputation score for this node
func (c *Client) SetReputationScore(score string) {
	c.metricsMutex.Lock()
	defer c.metricsMutex.Unlock()
	c.metrics.ReputationScore = score
}

// GetMetrics returns the current metrics
func (c *Client) GetMetrics() *NodeMetrics {
	c.metricsMutex.RLock()
	defer c.metricsMutex.RUnlock()

	// Return a copy to avoid race conditions
	return &NodeMetrics{
		LatencyMs:       c.metrics.LatencyMs,
		JitterMs:        c.metrics.JitterMs,
		PacketLoss:      c.metrics.PacketLoss,
		QueueDepth:      c.metrics.QueueDepth,
		PricePerGB:      c.metrics.PricePerGB,
		ReputationScore: c.metrics.ReputationScore,
		Capabilities:    append([]string{}, c.metrics.Capabilities...),
		LastUpdate:      c.metrics.LastUpdate,
	}
}

// GetBeaconMetadata returns the beacon metadata for inclusion in cluster state
func (c *Client) GetBeaconMetadata() *pb.BeaconMetadata {
	c.metricsMutex.RLock()
	defer c.metricsMutex.RUnlock()

	return &pb.BeaconMetadata{
		BeaconNodeId:     c.nodeID,
		NodeSignature:    c.publicKey,
		Timestamp:        time.Now().Unix(),
		ReputationScore:  c.metrics.ReputationScore,
		NodeCapabilities: c.metrics.Capabilities,
		LatencyMs:        c.metrics.LatencyMs,
		JitterMs:         c.metrics.JitterMs,
		PacketLoss:       c.metrics.PacketLoss,
		QueueDepth:       c.metrics.QueueDepth,
		PricePerGb:       c.metrics.PricePerGB,
	}
}

// Close closes the beacon client
func (c *Client) Close() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	c.connected = false
	c.logger.Info("beacon client closed")

	return nil
}

// GetNodeID returns the node ID
func (c *Client) GetNodeID() string {
	return c.nodeID
}
