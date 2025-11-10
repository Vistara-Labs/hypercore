# Hypercore Architecture Analysis & IBRL Integration Plan

## 1. KEY DIRECTORIES & ORGANIZATION

### Core Structure
```
/home/user/hypercore/
├── cmd/                           # Entry points
│   └── containerd-shim-hypercore-example/main.go
├── internal/hypercore/            # CLI implementation
│   ├── commands.go               # Command definitions
│   ├── config.go                 # Configuration struct
│   ├── flags.go                  # Flag definitions
│   └── main.go                   # Entry point
├── pkg/                          # Core packages
│   ├── cluster/                  # Cluster orchestration (PRIMARY FOCUS)
│   │   ├── serf.go              # Serf agent + gossip protocol (883 lines)
│   │   ├── service.go           # gRPC/HTTP servers (98 lines)
│   │   ├── proxy.go             # Reverse proxy for routing (120 lines)
│   │   └── utils.go             # Helper functions (30 lines)
│   ├── containerd/              # Containerd integration
│   │   ├── repo.go              # Container lifecycle management
│   │   └── config.go            # Configuration
│   ├── hypervisor/              # Hypervisor abstraction layer
│   │   ├── firecracker/         # Firecracker implementation
│   │   ├── cloudhypervisor/     # Cloud Hypervisor implementation
│   │   └── shared/              # Common interfaces
│   ├── models/                  # Data models
│   │   ├── microvm.go           # MicroVM specifications
│   │   └── network.go           # Network models
│   ├── network/                 # Network utilities
│   ├── proto/cluster/           # Protocol Buffers definitions
│   │   └── cluster.proto        # Service definitions
│   ├── containerd/              # Containerd wrapper
│   ├── defaults/                # Default values
│   ├── ports/                   # Port management
│   └── shim/                    # Containerd shim
└── docs/
    ├── cluster.md               # Cluster operations guide
    └── spawning.md              # Workload spawning guide
```

### Line Counts
- serf.go: 883 lines (core agent logic)
- proxy.go: 120 lines (HTTP reverse proxy)
- service.go: 98 lines (gRPC/HTTP handlers)
- cluster module total: ~1,131 lines

---

## 2. CLI COMMANDS & ENTRY POINTS

### Main Commands (in `internal/hypercore/commands.go`)
1. **`hypercore cluster [join-addr]`** - Main cluster operation command
   - Starts the cluster agent
   - Listens on gRPC (default :8000) and HTTP (default :8001)
   - Binds Serf on port 7946

2. **`hypercore cluster spawn`** - Spawn workload
   - Requires: CPU, memory, image reference, optional ports, environment variables
   - Uses gRPC to send spawn request to local agent

3. **`hypercore cluster stop`** - Stop workload
   - Requires: workload ID
   - Uses gRPC to send stop request

4. **`hypercore cluster list`** - List all workloads in cluster
   - Uses gRPC to query all nodes

5. **`hypercore cluster logs`** - Get workload logs
   - Requires: workload ID
   - Uses gRPC to locate workload, then HTTP to fetch logs

### Configuration Structure
```go
type Config struct {
    CtrSocketPath        string  // Containerd socket
    CtrNamespace         string  // Containerd namespace
    DefaultVMProvider    string  // firecracker/cloudhypervisor/docker
    ClusterBindAddr      string  // Serf bind address
    ClusterBaseURL       string  // Base domain for workloads
    ClusterTLSCert       string  // TLS certificate path
    ClusterTLSKey        string  // TLS key path
    GrpcBindAddr         string  // gRPC server bind address
    HTTPBindAddr         string  // HTTP server bind address
    RespawnOnNodeFailure bool    // Auto-reschedule on node failure
}
```

### Flags (in `internal/hypercore/flags.go`)
- `--cluster-bind-addr`: Serf gossip bind address (default `:7946`)
- `--cluster-base-url`: Domain for exposing workloads
- `--cluster-tls-cert/key`: HTTPS certificate/key for reverse proxy
- `--grpc-bind-addr`: gRPC server (default `0.0.0.0:8000`)
- `--http-bind-addr`: HTTP server (default `0.0.0.0:8001`)
- `--respawn-on-node-failure`: Enable workload rescheduling

---

## 3. SERF GOSSIP MESSAGE STRUCTURE

### Protocol Buffers Definition (`pkg/proto/cluster.proto`)
```protobuf
enum ClusterEvent {
    ERROR = 0;
    SPAWN = 1;
    STOP = 2;
}

message ClusterMessage {
    ClusterEvent event = 1;
    google.protobuf.Any wrappedMessage = 2;
}

message VmSpawnRequest {
    uint32 cores = 1;
    uint32 memory = 2;
    string image_ref = 3;
    map<uint32, uint32> ports = 4;  // host -> container ports
    bool dry_run = 5;
    repeated string env = 6;
}

message VmStopRequest {
    string id = 1;
}

message WorkloadState {
    string id = 1;
    VmSpawnRequest source_request = 2;
}

message NodeStateResponse {
    Node node = 1;
    repeated WorkloadState workloads = 2;
}
```

### Serf Configuration (in `pkg/cluster/serf.go`)
```go
cfg.MemberlistConfig.GossipInterval = time.Second * 2
cfg.MemberlistConfig.ProbeInterval = time.Second * 5
cfg.MemberlistConfig.SuspicionMult = 6
cfg.MemberlistConfig.GossipNodes = 2
cfg.UserEventSizeLimit = 2048
MaxQueueDepth = 5000
WorkloadBroadcastPeriod = time.Second * 5
```

### Event Handling Flow
1. **Spawn/Stop Queries**: Serf Query RPC (request/response)
   - First: Dry-run query to all nodes to check capacity
   - Second: Actual spawn query to first responsive node

2. **State Broadcasts**: Serf UserEvents (gossip)
   - Every 5 seconds, broadcast workload state
   - Handles large states via batching (10 workloads per message)
   - Uses ID-based fragmentation: "begin", "part", "finish", "complete"

3. **State Detection**: Hash-based change detection
   - Creates SHA256 hash of workload ID list
   - Only broadcasts if state changes

---

## 4. WORKLOAD SCHEDULING

### Current Algorithm (in `pkg/cluster/serf.go` - `SpawnRequest()`)
**First-Fit Bin Packing with Dry-Run**:
1. Broadcast dry-run spawn request to all cluster nodes
2. First node to respond is selected
3. Send actual spawn request to that node only
4. Uses `runtime.NumCPU()` and available memory for capacity checking

### Constraints Checked:
```go
// vCPU limit: max of physical CPU count or 225
if (vcpuUsed + int(payload.GetCores())) > max(runtime.NumCPU(), 225)
    return error

// Memory limit: check available RAM
if (memUsed + int(payload.GetMemory())) > int(availableMem)
    return warning
```

### Limitations
- No resource reservations (first-fit doesn't account for pending requests)
- No affinity/anti-affinity policies
- No priority queuing
- No intelligent scheduling based on node metrics

---

## 5. REVERSE PROXY & ROUTING LOGIC

### Architecture (`pkg/cluster/proxy.go`)
```
External: https://497b6ad8.deployments.vistara.dev:443
                    ↓ (Host Header matching)
Internal Reverse Proxy (HTTP handler)
                    ↓ (Container ID lookup)
Backend: 192.168.127.15:8080
```

### Implementation
1. **Service Registry**: Maps container IDs to addresses
   ```go
   serviceIDPortMaps map[string]map[uint32]string
   // Map: containerID -> (hostPort -> containerAddr)
   ```

2. **Dynamic Registration** (in `monitorWorkloads()`)
   - Every 5 seconds, discover running containers
   - Extract port mappings from spawn request labels
   - Register with proxy if not already registered

3. **HTTP Handler** (in `NewServiceProxy()`)
   - Extracts container ID from Host header
   - Looks up container address
   - Creates reverse proxy to backend
   - Supports TLS for host ports

4. **Port Listening**
   - One listener per proxied port
   - Dynamic listener creation
   - Cleanup on service deregistration

---

## 6. CONTAINERD INTEGRATION

### Key Integration Points (`pkg/containerd/repo.go`)

#### Container Creation Flow
1. **Pull Image** - Download from registry
2. **Create Network Namespace** - Isolated network for container
3. **Configure Spec** - Resource limits, environment, mounts
4. **Add CNI Networks** - ptp (point-to-point), firewall, tc-redirect-tap
5. **Create Task** - Start container
6. **Launch Task** - Execute entrypoint

#### Container Lifecycle Operations
```go
// Repo interface
func (r *Repo) CreateContainer(ctx, opts CreateContainerOpts) (id string, err error)
func (r *Repo) GetTasks(ctx) ([]*task.Process, error)
func (r *Repo) GetTask(ctx, id string) (*task.Process, error)
func (r *Repo) GetContainer(ctx, id string) (containerd.Container, error)
func (r *Repo) DeleteContainer(ctx, id string) (exitCode uint32, err error)
func (r *Repo) GetContainerPrimaryIP(ctx, id string) (ip string, error)
func (r *Repo) Attach(ctx, id string) error  // Attach to container
```

#### Resource Limiting
```go
// CPU and Memory limits applied via OCI spec
oci.WithMemoryLimit(opts.Limits.MemoryBytes)
oci.WithCPUCFS(int64(cpuFraction*100000), 100000)
```

#### Network Configuration
- **CNI Plugins Used**:
  - `ptp`: Point-to-point networking
  - `firewall`: Network isolation
  - `tc-redirect-tap`: TAP device redirection (for VMs)

- **Network Namespace**: Each container gets isolated network namespace
- **IP Assignment**: Host-local IPAM (subnet: 192.168.127.0/24)
- **DNS**: Uses host's /etc/resolv.conf

### Spawned Container Structure
```go
type CreateContainerOpts struct {
    ImageRef      string           // Container/VM image
    Snapshotter   string           // devmapper or empty for runc
    Runtime       {Name, Options}  // Runtime (runc, hypercore.example)
    Limits        {CPUFraction, MemoryBytes}
    CioCreator    cio.Creator      // I/O configuration
    Labels        map[string]string
    Env           []string
}
```

---

## 7. MONITORING & METRICS INFRASTRUCTURE

### Prometheus Metrics (in `pkg/cluster/serf.go`)

#### Metrics Registered
```go
// Gauge metrics (current values)
hypercore_serf_queue_depth     // Serf event queue depth
hypercore_workload_count       // Running workloads per node

// Counter metrics (monotonic)
hypercore_broadcast_skipped_total  // Failed broadcasts due to queue depth
hypercore_state_changes_total      // State changes detected
```

#### Monitoring Logic
1. **Queue Depth Monitoring** (in `monitorWorkloads()`)
   - Checks Serf stats every 5 seconds
   - Skips broadcast if queue depth > 10,000
   - Logs warnings for queue depth > 1,000

2. **State Change Detection** (in `monitorWorkloads()`)
   - Creates SHA256 hash of workload list
   - Only broadcasts if hash changes
   - Reduces gossip overhead

3. **Workload Tracking**
   - Monitors container status every 5 seconds
   - Auto-respawns failed containers (if enabled)
   - Registers port mappings with reverse proxy

4. **Node Failure Detection** (in `monitorStateUpdates()`)
   - Tracks last update from each node
   - Marks node as dead if no update for 15 seconds
   - Optional: auto-reschedules workloads (if `respawn-on-node-failure` enabled)

---

## 8. ARCHITECTURE FLOW DIAGRAM

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLI User Commands                            │
│  spawn | stop | list | logs | attach                           │
└──────────────────────┬──────────────────────────────────────────┘
                       │
        ┌──────────────┴──────────────┐
        │                             │
        v                             v
   gRPC Client              Cluster Agent (ClusterCommand)
   (spawn/stop)             │
                            ├─ EventCh (Serf events)
                            ├─ ServiceProxy (HTTP routing)
                            ├─ Containerd Repo (lifecycle)
                            ├─ Serf Agent
                            │  ├─ Query Handler (spawn/stop requests)
                            │  └─ Event Handler (gossip)
                            ├─ monitorWorkloads() goroutine
                            │  ├─ Gets container list every 5s
                            │  ├─ Broadcasts state via Serf UserEvent
                            │  └─ Registers with ServiceProxy
                            └─ monitorStateUpdates() goroutine
                               └─ Detects node failures

           Serf Gossip Network (Port 7946)
    ┌─────────────────────────────────────────┐
    │ Node A              Node B              │
    │ ┌──────────┐      ┌──────────┐         │
    │ │ Agent    │      │ Agent    │         │
    │ │ +Serf    │──────│ +Serf    │         │
    │ │ +CTR     │      │ +CTR     │         │
    │ └──────────┘      └──────────┘         │
    └─────────────────────────────────────────┘

    gRPC Ports (per node)
    ├─ 8000: spawn/stop/list/logs
    └─ 8001: HTTP logs + reverse proxy

    HTTP Reverse Proxy
    ├─ Host: containerID.domain
    └─ → Backend: container:port
```

---

## 9. RECOMMENDED IBRL INTEGRATION POINTS

### 9.1 Beacon Module Integration
**Location**: New package `/home/user/hypercore/pkg/beacon/`

**Integration Points**:
1. **Node Discovery Enhancement**
   - Extend Serf Agent to include beacon metadata
   - Publish node capabilities, location, reputation in gossip messages
   - Add beacon node status to Node protobuf message

2. **Workload Advertisement**
   - Include beacon-signed workload manifest in NodeStateResponse
   - Broadcast workload capabilities alongside state

**Changes Required**:
- Add fields to `ClusterMessage` proto for beacon metadata
- Extend `Agent` struct with beacon client
- Modify `monitorWorkloads()` to include beacon signatures

### 9.2 Policy Module Integration
**Location**: New package `/home/user/hypercore/pkg/policy/`

**Integration Points**:
1. **Scheduling Policy Enforcement** (replaces current first-fit)
   - Intercept `SpawnRequest()` with policy engine
   - Evaluate node suitability based on policies
   - Return policy-compliant nodes for scheduling

2. **Permission Checks**
   - Add policy validation to `handleSpawnRequest()` and `handleStopRequest()`
   - Check image whitelist, resource quotas, user permissions

3. **Service Port Policy**
   - Validate exposed ports against policy
   - Enforce network isolation policies

**Changes Required**:
- Add `PolicyEngine` to Agent struct
- Wrap spawn/stop handlers with policy checks
- Store policies in distributed config (via Serf or external store)

### 9.3 Proof Module Integration
**Location**: New package `/home/user/hypercore/pkg/proof/`

**Integration Points**:
1. **Workload Execution Proof**
   - Collect container metrics from Firecracker/Cloud Hypervisor
   - Generate proof of computation
   - Append proof to workload logs

2. **State Attestation**
   - Sign NodeStateResponse with proof key
   - Include proof hash in broadcast messages

3. **Proof Aggregation**
   - Collect proofs from all nodes
   - Verify against beacon registry

**Changes Required**:
- Hook into `monitorWorkloads()` to collect metrics
- Sign state changes in `monitorWorkloads()`
- Add proof field to WorkloadState protobuf

### 9.4 Proto Changes for IBRL
```protobuf
// Add to cluster.proto
message BeaconMetadata {
    string beacon_node_id = 1;
    bytes beacon_signature = 2;
    int64 timestamp = 3;
}

message PolicyContext {
    string policy_id = 1;
    repeated string tags = 2;
}

message WorkloadProof {
    string proof_hash = 1;
    bytes proof_signature = 2;
    map<string, string> metrics = 3;
}

// Extend existing messages
message WorkloadState {
    string id = 1;
    VmSpawnRequest source_request = 2;
    WorkloadProof proof = 3;  // NEW
}

message NodeStateResponse {
    Node node = 1;
    repeated WorkloadState workloads = 2;
    BeaconMetadata beacon = 3;  // NEW
    PolicyContext policy = 4;   // NEW
}
```

---

## 10. EXISTING CODE PATTERNS FOR IBRL

### Pattern 1: Goroutine-based Monitoring
Current pattern in `monitorWorkloads()` and `monitorStateUpdates()`:
```go
func (a *Agent) monitorWorkloads() {
    ticker := time.NewTicker(WorkloadBroadcastPeriod)
    for range ticker.C {
        // Periodic work
    }
}
```

**Use for**: Beacon heartbeats, proof generation, policy refresh

### Pattern 2: Serf Event Broadcasting
Current pattern for state broadcasts:
```go
marshaled, err := proto.Marshal(&partResp)
if err := a.serf.UserEvent(StateBroadcastEvent, marshaled, true); err != nil {
    a.logger.WithError(err).Error("failed to broadcast")
}
```

**Use for**: Broadcasting beacon attestations, proof confirmations

### Pattern 3: Handler Registration
Current pattern for query handling:
```go
switch baseMessage.GetEvent() {
case pb.ClusterEvent_SPAWN:
    response, err = a.handleSpawnRequest(&payload)
case pb.ClusterEvent_STOP:
    response, err = a.handleStopRequest(&payload)
}
```

**Use for**: New IBRL event types (e.g., `ClusterEvent_PROOF_VERIFY`)

### Pattern 4: Metrics Registration
Current pattern for Prometheus metrics:
```go
prometheus.MustRegister(gauge, counter)
```

**Use for**: IBRL-specific metrics (beacon connections, policy violations, proof latency)

---

## 11. DEPLOYMENT ARCHITECTURE

### Node Bootstrap Sequence
1. Start hypercore cluster agent
2. Bind to Serf port (7946)
3. Join existing cluster (optional)
4. Start gRPC server (8000)
5. Start HTTP server (8001) with reverse proxy
6. Launch monitoring goroutines:
   - `monitorWorkloads()` - broadcasts state every 5 seconds
   - `monitorStateUpdates()` - detects failures every 5 seconds
   - `Handler()` - processes Serf events

### Service Registration Flow
1. **User Request** → spawn command
2. **CLI** → gRPC to local agent (Spawn RPC)
3. **Agent** → Serf dry-run query to all nodes
4. **Node Responses** → first responder selected
5. **Actual Spawn** → Serf query to selected node
6. **Node Handler** → creates container via Containerd
7. **Container Start** → monitorWorkloads picks it up
8. **State Broadcast** → gossip state update with port mappings
9. **Proxy Registration** → register in reverse proxy
10. **User Access** → https://containerID.domain:443 → 192.168.127.X:port

---

## 12. KEY METRICS FOR MONITORING

### Serf Health
- `hypercore_serf_queue_depth` - Event queue depth (currently monitored)
- Member join/leave events
- Network latency between nodes

### Workload Health
- `hypercore_workload_count` - Active workloads per node
- Container restart count
- Resource utilization (CPU, memory)

### Cluster Health
- `hypercore_state_changes_total` - State updates
- `hypercore_broadcast_skipped_total` - Failed broadcasts
- Node responsiveness to queries
- Spawn success/failure rate

---

## 13. SECURITY CONSIDERATIONS

### Current Limitations
- No encryption in Serf gossip
- TLS only on reverse proxy
- No container image verification
- No secrets management
- Network not fully isolated (guests can access host ports)
- No workload state persistence

### For IBRL Integration
- Use beacon for node authentication
- Use policy for resource isolation
- Use proof for computation verification
- Consider extending TLS to Serf gossip
- Add image signature verification

---

## 14. PERFORMANCE CHARACTERISTICS

### Latency
- Spawn latency: ~90 seconds (allow time for image pull + startup)
- State broadcast: 5 second intervals
- Serf gossip: 2 second intervals

### Throughput
- Single node: Limited by Firecracker/Cloud Hypervisor startup
- Cluster: Serf queue depth limit of 5000 events
- Broadcast batching: 10 workloads per message

### Resource Usage
- Serf gossip nodes: 2 (conservative)
- Memory per workload: configurable (512MB-32GB)
- CPU per workload: up to node capacity

---

## 15. NEXT STEPS FOR IBRL INTEGRATION

1. **Phase 1: Beacon**
   - Create beacon client wrapper
   - Add beacon metadata to Node protobuf
   - Implement node attestation in `monitorWorkloads()`

2. **Phase 2: Policy**
   - Create policy engine
   - Replace first-fit scheduler with policy-based scheduler
   - Add policy validation hooks

3. **Phase 3: Proof**
   - Hook into Firecracker/Cloud Hypervisor metrics
   - Implement proof generation
   - Add proof aggregation and verification

4. **Phase 4: Integration Testing**
   - Multi-node cluster tests
   - IBRL scenario testing
   - Performance benchmarking
