package beacon

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// AttestationType defines the type of attestation
type AttestationType string

const (
	// NodeIdentityAttestation attests to the node's identity
	NodeIdentityAttestation AttestationType = "NODE_IDENTITY"

	// StateHashAttestation attests to the node's current state hash
	StateHashAttestation AttestationType = "STATE_HASH"

	// WorkloadAttestation attests to a specific workload
	WorkloadAttestation AttestationType = "WORKLOAD"
)

// Attestation represents a cryptographic attestation
type Attestation struct {
	Type      AttestationType
	NodeID    string
	Data      []byte
	Signature []byte
	Timestamp int64
	PublicKey []byte
}

// AttestationVerifier verifies attestations
type AttestationVerifier struct {
	logger *logrus.Logger
}

// NewAttestationVerifier creates a new attestation verifier
func NewAttestationVerifier(logger *logrus.Logger) *AttestationVerifier {
	return &AttestationVerifier{
		logger: logger,
	}
}

// GenerateAttestation generates a new attestation
func GenerateAttestation(
	ctx context.Context,
	attestationType AttestationType,
	nodeID string,
	data []byte,
	privateKey ed25519.PrivateKey,
	publicKey ed25519.PublicKey,
) (*Attestation, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("attestation data cannot be empty")
	}

	timestamp := time.Now().Unix()

	// Create attestation message: type|nodeID|timestamp|data
	message := fmt.Sprintf("%s|%s|%d|%s", attestationType, nodeID, timestamp, hex.EncodeToString(data))

	// Sign the message
	signature := ed25519.Sign(privateKey, []byte(message))

	return &Attestation{
		Type:      attestationType,
		NodeID:    nodeID,
		Data:      data,
		Signature: signature,
		Timestamp: timestamp,
		PublicKey: publicKey,
	}, nil
}

// Verify verifies an attestation signature
func (v *AttestationVerifier) Verify(ctx context.Context, attestation *Attestation) (bool, error) {
	if attestation == nil {
		return false, fmt.Errorf("attestation cannot be nil")
	}

	if len(attestation.PublicKey) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key size: %d", len(attestation.PublicKey))
	}

	// Reconstruct the signed message
	message := fmt.Sprintf("%s|%s|%d|%s",
		attestation.Type,
		attestation.NodeID,
		attestation.Timestamp,
		hex.EncodeToString(attestation.Data),
	)

	// Verify signature
	publicKey := ed25519.PublicKey(attestation.PublicKey)
	valid := ed25519.Verify(publicKey, []byte(message), attestation.Signature)

	if valid {
		v.logger.WithFields(logrus.Fields{
			"type":    attestation.Type,
			"node_id": attestation.NodeID,
		}).Debug("attestation verified successfully")
	} else {
		v.logger.WithFields(logrus.Fields{
			"type":    attestation.Type,
			"node_id": attestation.NodeID,
		}).Warn("attestation verification failed")
	}

	return valid, nil
}

// VerifyWithAge verifies an attestation and checks if it's not too old
func (v *AttestationVerifier) VerifyWithAge(ctx context.Context, attestation *Attestation, maxAge time.Duration) (bool, error) {
	// First verify the signature
	valid, err := v.Verify(ctx, attestation)
	if err != nil || !valid {
		return false, err
	}

	// Check age
	now := time.Now().Unix()
	age := now - attestation.Timestamp

	if time.Duration(age)*time.Second > maxAge {
		v.logger.WithFields(logrus.Fields{
			"type":      attestation.Type,
			"node_id":   attestation.NodeID,
			"age":       age,
			"max_age":   maxAge.Seconds(),
		}).Warn("attestation is too old")
		return false, fmt.Errorf("attestation is too old: %ds > %ds", age, int(maxAge.Seconds()))
	}

	return true, nil
}

// HashStateData creates a hash of state data for attestation
func HashStateData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// AttestNodeIdentity creates a node identity attestation
func AttestNodeIdentity(
	ctx context.Context,
	nodeID string,
	privateKey ed25519.PrivateKey,
	publicKey ed25519.PublicKey,
) (*Attestation, error) {
	// Use the node ID as the data for identity attestation
	data := []byte(nodeID)

	return GenerateAttestation(
		ctx,
		NodeIdentityAttestation,
		nodeID,
		data,
		privateKey,
		publicKey,
	)
}

// AttestStateHash creates a state hash attestation
func AttestStateHash(
	ctx context.Context,
	nodeID string,
	stateData []byte,
	privateKey ed25519.PrivateKey,
	publicKey ed25519.PublicKey,
) (*Attestation, error) {
	// Hash the state data
	hash := HashStateData(stateData)

	return GenerateAttestation(
		ctx,
		StateHashAttestation,
		nodeID,
		hash,
		privateKey,
		publicKey,
	)
}

// AttestWorkload creates a workload attestation
func AttestWorkload(
	ctx context.Context,
	nodeID string,
	workloadID string,
	privateKey ed25519.PrivateKey,
	publicKey ed25519.PublicKey,
) (*Attestation, error) {
	// Use workload ID as the data
	data := []byte(workloadID)

	return GenerateAttestation(
		ctx,
		WorkloadAttestation,
		nodeID,
		data,
		privateKey,
		publicKey,
	)
}
