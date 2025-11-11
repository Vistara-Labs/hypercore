# IBRL Integration Checklist & Decision Points

## 1. PROTO CHANGES REQUIRED

### Add to `/home/user/hypercore/pkg/proto/cluster.proto`

```protobuf
// Beacon module additions
message BeaconMetadata {
    string beacon_node_id = 1;           // Identity from beacon
    bytes node_signature = 2;             // Signature proof from beacon
    int64 timestamp = 3;                  // When registered
    string reputation_score = 4;          // From beacon registry
    repeated string node_capabilities = 5; // Advertise node features
}

// Policy module additions  
message PolicyContext {
    string policy_id = 1;                 // Which policy applied
    repeated string tags = 2;             // Policy tags/labels
    string enforcement_level = 3;         // PERMISSIVE, ENFORCE, DENY
}

// Proof module additions
message WorkloadProof {
    string proof_hash = 1;                // SHA256 of execution proof
    bytes proof_signature = 2;            // Signed proof
    map<string, string> metrics = 3;      // CPU%, Memory%, Time, etc.
    int64 proof_timestamp = 4;            // When proof generated
    string hypervisor_type = 5;           // firecracker/cloudhypervisor
}

// Extend existing messages
message WorkloadState {
    string id = 1;
    VmSpawnRequest source_request = 2;
    WorkloadProof proof = 3;              // NEW - proof of execution
}

message NodeStateResponse {
    Node node = 1;
    repeated WorkloadState workloads = 2;
    BeaconMetadata beacon = 3;            // NEW - beacon registration
    PolicyContext policy = 4;             // NEW - policy context
    string state_signature = 5;           // NEW - sign whole response
}

// New event types
enum ClusterEvent {
    ERROR = 0;
    SPAWN = 1;
    STOP = 2;
    BEACON_ATTEST = 3;                   // NEW
    POLICY_QUERY = 4;                    // NEW
    PROOF_VERIFY = 5;                    // NEW
}

// Beacon attestation message
message BeaconAttestation {
    string node_id = 1;
    string attestation_type = 2;          // "NODE_IDENTITY", "STATE_HASH"
    bytes attestation_data = 3;           // Beacon-signed data
}

// Policy query message
message PolicyQuery {
    string workload_id = 1;
    string policy_type = 2;               // "SCHEDULING", "PERMISSION", "NETWORK"
    map<string, string> context = 3;      // Additional context
}

// Proof verification message
message ProofVerification {
    string workload_id = 1;
    string proof_hash = 1;
    string verification_result = 2;       // "VALID", "INVALID", "UNKNOWN"
    string beacon_registry_hash = 3;      // Cross-check with registry
}
```

**Generation Command**: 
```bash
cd /home/user/hypercore
protoc --go_out=. --go-grpc_out=. --proto_path=. pkg/proto/cluster.proto
```

---

## 2. PACKAGE STRUCTURE FOR IBRL

### New Packages to Create

```
/home/user/hypercore/pkg/
├── beacon/                           # NEW - Beacon module
│   ├── client.go                     # Beacon node client wrapper
│   ├── registry.go                   # Registry interface
│   └── attestation.go                # Attestation logic
│
├── policy/                           # NEW - Policy module  
│   ├── engine.go                     # Policy evaluation engine
│   ├── scheduler.go                  # Policy-based scheduler
│   ├── store.go                      # Policy storage interface
│   └── evaluator.go                  # Policy evaluation logic
│
└── proof/                            # NEW - Proof module
    ├── collector.go                  # Metrics collection
    ├── generator.go                  # Proof generation
    ├── verifier.go                   # Proof verification
    └── aggregator.go                 # Cross-node verification
```

---

## 3. INTEGRATION POINTS IN EXISTING CODE

### Point 1: Agent Initialization (serf.go - NewAgent)
**File**: `/home/user/hypercore/pkg/cluster/serf.go`

```go
// CHANGE: Add IBRL components to Agent struct
type Agent struct {
    // ... existing fields ...
    
    // IBRL Components
    beaconClient    beacon.Client           // NEW
    policyEngine    policy.Engine           // NEW
    proofCollector  proof.Collector         // NEW
    
    // NEW metrics
    ibrlBeaconConnectionStatus prometheus.Gauge
    ibrlPolicyViolations       prometheus.Counter
    ibrlProofLatency           prometheus.Histogram
}

// CHANGE: In NewAgent() function, initialize IBRL components
func NewAgent(...) (*Agent, error) {
    // ... existing serf initialization ...
    
    // NEW: Initialize IBRL
    beaconClient, err := beacon.NewClient(logger, cfg.BeaconEndpoint)
    if err != nil {
        return nil, fmt.Errorf("failed to init beacon: %w", err)
    }
    
    policyEngine, err := policy.NewEngine(logger, cfg.PolicyStore)
    if err != nil {
        return nil, fmt.Errorf("failed to init policy: %w", err)
    }
    
    proofCollector, err := proof.NewCollector(logger, hypervisorType)
    if err != nil {
        return nil, fmt.Errorf("failed to init proof collector: %w", err)
    }
    
    agent.beaconClient = beaconClient
    agent.policyEngine = policyEngine
    agent.proofCollector = proofCollector
    
    // NEW: Start IBRL goroutines
    go agent.beaconHeartbeat()
    go agent.proofGenerator()
    
    return agent, nil
}
```

### Point 2: Spawn Request Handling (serf.go - SpawnRequest)
**File**: `/home/user/hypercore/pkg/cluster/serf.go`

```go
// CHANGE: Wrap scheduling logic with policy
func (a *Agent) SpawnRequest(req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
    // NEW: Policy validation FIRST
    if allowed, reason := a.policyEngine.CanSpawn(req); !allowed {
        return nil, fmt.Errorf("policy violation: %s", reason)
    }
    
    // CHANGE: Use policy-based scheduling instead of first-fit
    // OLD: dry-run query to all nodes, take first responder
    // NEW: policy engine evaluates nodes based on policy rules
    
    nodes, err := a.policyEngine.SelectNodes(req, a.serf.Members())
    if err != nil {
        return nil, err
    }
    
    // NEW: Try policy-selected nodes in order
    for _, nodeName := range nodes {
        params := a.serf.DefaultQueryParams()
        params.Timeout = time.Second * 90
        params.FilterNodes = []string{nodeName}
        
        query, err := a.serf.Query(QueryName, payload, params)
        if err != nil {
            continue
        }
        
        // ... response handling ...
    }
    
    return nil, errors.New("no suitable node found")
}
```

### Point 3: Handle Spawn Request (serf.go - handleSpawnRequest)
**File**: `/home/user/hypercore/pkg/cluster/serf.go`

```go
// CHANGE: Add proof collection hook
func (a *Agent) handleSpawnRequest(payload *pb.VmSpawnRequest) (ret []byte, retErr error) {
    // ... capacity check ...
    
    id, err := a.ctrRepo.CreateContainer(ctx, vcontainerd.CreateContainerOpts{
        // ... container config ...
    })
    if err != nil {
        return nil, err
    }
    
    // NEW: Start proof collection for this workload
    a.proofCollector.StartCollection(id, payload.GetImageRef())
    defer func() {
        if retErr == nil {
            proof, err := a.proofCollector.GenerateProof(id)
            if err != nil {
                a.logger.WithError(err).Warn("failed to generate proof")
            } else {
                // Store proof for later broadcast
                a.storeProof(id, proof)
            }
        }
    }()
    
    // ... rest of spawn logic ...
}
```

### Point 4: Monitor Workloads (serf.go - monitorWorkloads)
**File**: `/home/user/hypercore/pkg/cluster/serf.go`

```go
// CHANGE: Add beacon attestation and proof signing to state broadcast
func (a *Agent) monitorWorkloads() {
    ticker := time.NewTicker(WorkloadBroadcastPeriod)
    for range ticker.C {
        // ... existing workload gathering ...
        
        resp := pb.NodeStateResponse{
            Node: &pb.Node{
                Id: a.serf.LocalMember().Name,
            },
        }
        
        for _, task := range tasks {
            // ... existing workload state ...
            
            workloadState := &pb.WorkloadState{
                Id: container.ID(),
                SourceRequest: &labelPayload,
            }
            
            // NEW: Attach proof if available
            if proof, ok := a.getStoredProof(container.ID()); ok {
                workloadState.Proof = proof
            }
            
            resp.Workloads = append(resp.Workloads, workloadState)
        }
        
        // NEW: Get beacon attestation
        if attestation, err := a.beaconClient.Attest(ctx, resp.Node.Id); err == nil {
            resp.Beacon = &pb.BeaconMetadata{
                BeaconNodeId: attestation.NodeID,
                NodeSignature: attestation.Signature,
                Timestamp: time.Now().Unix(),
            }
        }
        
        // NEW: Sign entire response
        signature := a.signNodeStateResponse(&resp)
        resp.StateSignature = signature
        
        // ... rest of broadcast logic ...
    }
}
```

### Point 5: Add New Serf Event Handlers (serf.go - Handler)
**File**: `/home/user/hypercore/pkg/cluster/serf.go`

```go
// CHANGE: Add handlers for IBRL event types
func (a *Agent) Handler() {
    for event := range a.eventCh {
        switch event.EventType() {
        // ... existing cases ...
        
        case serf.EventQuery:
            query := event.(*serf.Query)
            var baseMessage pb.ClusterMessage
            if err := proto.Unmarshal(query.Payload, &baseMessage); err != nil {
                continue
            }
            
            var response []byte
            var err error
            
            switch baseMessage.GetEvent() {
            // ... existing SPAWN/STOP cases ...
            
            case pb.ClusterEvent_BEACON_ATTEST:            // NEW
                var payload pb.BeaconAttestation
                if err := baseMessage.GetWrappedMessage().UnmarshalTo(&payload); err != nil {
                    continue
                }
                response, err = a.handleBeaconAttest(&payload)
                
            case pb.ClusterEvent_POLICY_QUERY:             // NEW
                var payload pb.PolicyQuery
                if err := baseMessage.GetWrappedMessage().UnmarshalTo(&payload); err != nil {
                    continue
                }
                response, err = a.handlePolicyQuery(&payload)
                
            case pb.ClusterEvent_PROOF_VERIFY:             // NEW
                var payload pb.ProofVerification
                if err := baseMessage.GetWrappedMessage().UnmarshalTo(&payload); err != nil {
                    continue
                }
                response, err = a.handleProofVerify(&payload)
            }
            
            if err := query.Respond(response); err != nil {
                a.logger.WithError(err).Error("failed to respond to query")
            }
        }
    }
}
```

### Point 6: Add CLI Flags (flags.go)
**File**: `/home/user/hypercore/internal/hypercore/flags.go`

```go
// ADD: New IBRL-related flags
const (
    // ... existing flags ...
    beaconEndpointFlag   = "beacon-endpoint"
    policyStoreFlag      = "policy-store"
    proofKeyPathFlag     = "proof-key-path"
    policyModeFlag       = "policy-mode"  // permissive/enforce
)

func AddIBRLFlags(cmd *cobra.Command, cfg *Config) {
    cmd.Flags().StringVar(&cfg.BeaconEndpoint,
        beaconEndpointFlag,
        "",
        "Beacon node endpoint (host:port)")
    
    cmd.Flags().StringVar(&cfg.PolicyStore,
        policyStoreFlag,
        "",
        "Policy storage endpoint")
    
    cmd.Flags().StringVar(&cfg.ProofKeyPath,
        proofKeyPathFlag,
        "",
        "Path to proof signing key")
    
    cmd.Flags().StringVar(&cfg.PolicyMode,
        policyModeFlag,
        "permissive",
        "Policy enforcement mode: permissive/enforce")
}

// CHANGE: Add to Config struct
type Config struct {
    // ... existing fields ...
    
    // IBRL Configuration
    BeaconEndpoint  string
    PolicyStore     string
    ProofKeyPath    string
    PolicyMode      string
}
```

---

## 4. TESTING STRATEGY

### Unit Tests to Add
```
pkg/beacon/
├── client_test.go          # Test beacon client wrapper
└── attestation_test.go     # Test attestation generation

pkg/policy/
├── engine_test.go          # Test policy evaluation
├── scheduler_test.go       # Test policy-based scheduling
└── store_test.go           # Test policy retrieval

pkg/proof/
├── collector_test.go       # Test metrics collection
├── generator_test.go       # Test proof generation
└── verifier_test.go        # Test proof verification
```

### Integration Tests
```
tests/
├── ibrl_integration_test.go           # End-to-end IBRL test
├── policy_scheduling_test.go          # Policy-based spawn
├── beacon_registration_test.go        # Beacon node discovery
└── proof_verification_test.go         # Proof validation across nodes
```

---

## 5. CONFIGURATION EXAMPLE

### New Config File Format
```toml
# config.toml
[cluster]
bind-addr = "0.0.0.0:7946"
base-url = "example.com"

[ibrl]
# Beacon configuration
beacon-endpoint = "beacon-node.example.com:8000"
beacon-attestation-interval = "30s"
beacon-retry-interval = "5s"

# Policy configuration
policy-store = "distributed"  # or "local", "remote"
policy-store-endpoint = "policy-server.example.com:8001"
policy-mode = "enforce"  # or "permissive"
policy-refresh-interval = "60s"

# Proof configuration
proof-key-path = "/etc/hypercore/proof.key"
proof-generation-enabled = true
proof-verification-enabled = true
proof-aggregation-enabled = true
```

---

## 6. DEPLOYMENT CHECKLIST

### Pre-Deployment
- [ ] Extend proto definitions
- [ ] Regenerate protobuf code
- [ ] Create beacon client wrapper
- [ ] Create policy engine
- [ ] Create proof collector/generator
- [ ] Add new CLI flags
- [ ] Update Agent struct
- [ ] Implement Handler cases for new events

### Testing
- [ ] Unit tests pass for beacon module
- [ ] Unit tests pass for policy module
- [ ] Unit tests pass for proof module
- [ ] Integration test: beacon registration
- [ ] Integration test: policy-based scheduling
- [ ] Integration test: proof generation & verification
- [ ] Multi-node cluster test

### Deployment
- [ ] Beacon node running and accessible
- [ ] Policy store configured
- [ ] TLS certificates for proof signing
- [ ] Start first cluster node with IBRL
- [ ] Verify beacon registration
- [ ] Spawn workload with policy check
- [ ] Verify proof generation
- [ ] Join second node and verify cluster state

---

## 7. ROLLBACK STRATEGY

### If IBRL Integration Fails

1. **Beacon Down**: 
   - Code should handle missing beacon gracefully
   - Fall back to first-fit scheduling
   - Log warnings but don't crash

2. **Policy Engine Error**:
   - If policy check fails, default to permissive mode
   - Log violation but allow spawn
   - Alert operator to investigate

3. **Proof Generation Failure**:
   - Non-critical, continue operation
   - Log and metrics track failures
   - Workload still runs without proof

4. **Full Rollback**:
   - Disable IBRL via config flag
   - Revert proto changes in a new version
   - Maintain backward compatibility

---

## 8. MONITORING & OBSERVABILITY

### New Prometheus Metrics

```go
// Beacon metrics
ibrl_beacon_connected              // 1 = connected, 0 = disconnected
ibrl_beacon_attestations_total     // Total attestations generated
ibrl_beacon_attestation_latency_ms // Latency to get attestation

// Policy metrics
ibrl_policy_violations_total       // Workloads rejected by policy
ibrl_policy_evaluations_total      // Total policy evaluations
ibrl_policy_evaluation_latency_ms  // Time to evaluate policy

// Proof metrics
ibrl_proof_generation_latency_ms   // Time to generate proof
ibrl_proof_verifications_total     // Total proof verifications
ibrl_proof_verification_failures   // Failed verifications
ibrl_proof_aggregation_latency_ms  // Time to aggregate proofs
```

### Logging Guidelines
- Beacon connection/disconnection events
- Policy violations (with details)
- Proof generation start/completion
- Cross-node verification results

---

## 9. KNOWN ISSUES & MITIGATIONS

| Issue | Impact | Mitigation |
|-------|--------|-----------|
| Beacon latency | Slow scheduling | Cache attestations, use timeouts |
| Policy store unavailable | Can't evaluate policies | Fall back to permissive mode |
| Proof generation slow | High latency | Generate async, store for later |
| Large cluster state | Broadcast failures | Already batching in monitorWorkloads |
| Signature verification | CPU overhead | Cache signatures, batch verification |

---

## 10. PHASED ROLLOUT

### Phase 1: Beacon Integration (Week 1-2)
- Create beacon client wrapper
- Add beacon metadata to proto
- Implement node attestation in monitorWorkloads()
- Test beacon registration

### Phase 2: Policy Integration (Week 3-4)
- Create policy engine
- Implement policy-based scheduler
- Add policy event handler
- Test policy enforcement

### Phase 3: Proof Integration (Week 5-6)
- Create proof collector/generator
- Implement proof signing
- Add proof verification handler
- Test proof generation & verification

### Phase 4: Full Integration & Testing (Week 7-8)
- Multi-node cluster tests
- IBRL scenario testing
- Performance benchmarking
- Documentation & runbooks

