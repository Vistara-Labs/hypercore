# Hypercore Architecture - Quick Reference Guide

## File Locations Quick Lookup

| Component | File | Purpose |
|-----------|------|---------|
| CLI Commands | `/home/user/hypercore/internal/hypercore/commands.go` | spawn, stop, list, logs, attach commands |
| Configuration | `/home/user/hypercore/internal/hypercore/config.go` | Config struct definition |
| Flags | `/home/user/hypercore/internal/hypercore/flags.go` | CLI flag definitions |
| **Serf Agent** | `/home/user/hypercore/pkg/cluster/serf.go` | **Core cluster orchestration (883 lines)** |
| gRPC/HTTP Servers | `/home/user/hypercore/pkg/cluster/service.go` | Spawn/Stop/List/Logs handlers |
| **Reverse Proxy** | `/home/user/hypercore/pkg/cluster/proxy.go` | HTTP routing to workloads |
| Containerd Repo | `/home/user/hypercore/pkg/containerd/repo.go` | Container lifecycle operations |
| Proto Definitions | `/home/user/hypercore/pkg/proto/cluster.proto` | gRPC service & message definitions |
| Data Models | `/home/user/hypercore/pkg/models/microvm.go` | MicroVM spec structures |
| Defaults | `/home/user/hypercore/pkg/defaults/defaults.go` | Constants & default values |
| Network Utils | `/home/user/hypercore/pkg/network/utils.go` | Network helper functions |

## Key Ports & Services

| Port | Service | Purpose |
|------|---------|---------|
| 7946 | Serf Gossip | Cluster member discovery & communication |
| 8000 | gRPC | spawn/stop/list/logs RPC calls |
| 8001 | HTTP | Logs endpoint + Reverse proxy |
| Dynamic | HTTP (TLS) | Per-workload routing (443:8080, etc.) |

## Message Flow Diagrams

### Spawn Workload
```
User CLI
  ↓
gRPC: spawn(cores=2, mem=512, image=X, ports=443:8080)
  ↓
Agent.SpawnRequest()
  ↓
Serf Query: dry-run to all nodes
  ↓ (first to respond selected)
Serf Query: actual spawn to selected node
  ↓
Agent.handleSpawnRequest()
  ↓
Containerd: CreateContainer()
  ↓
monitorWorkloads() (next 5s tick)
  ↓
Serf UserEvent: broadcast state
  ↓
ServiceProxy.Register(443, containerID, 192.168.127.X:8080)
  ↓
User Access: https://containerID.domain:443
```

### State Synchronization
```
Every 5 seconds:
monitorWorkloads() runs
  ↓
Get all running containers from containerd
  ↓
Hash workload list
  ↓
If changed:
  ├─ Create NodeStateResponse
  ├─ Batch into 10-workload chunks
  ├─ Add fragmentation markers (begin/part/finish/complete)
  ├─ Marshal as protobuf
  └─ Broadcast via Serf UserEvent
  ↓
All nodes receive event
  ↓
Reassemble state
  ↓
Register services with proxy
  ↓
Update lastStateUpdate map
```

## Code Entry Points

### Starting a Cluster Node
```go
// File: internal/hypercore/main.go
func Run() {
    cmd := &cobra.Command{Use: "vs"}
    cmd.AddCommand(ClusterCommand(cfg))
    // ... attach, spawn, stop, list
    cmd.Execute()
}

// File: internal/hypercore/commands.go
func ClusterCommand(cfg *Config) *cobra.Command {
    // Creates Agent via cluster.NewAgent()
    // Starts: HTTP server, gRPC server, agent.Handler()
}
```

### Spawning a Workload
```go
// File: internal/hypercore/commands.go
func ClusterSpawnCommand(cfg *Config) *cobra.Command {
    // gRPC call to local agent:
    // pb.NewClusterServiceClient(conn).Spawn(context.Background(), &pb.VmSpawnRequest{...})
}

// File: pkg/cluster/service.go
func (s *server) Spawn(ctx, req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
    return s.agent.SpawnRequest(req)
}

// File: pkg/cluster/serf.go
func (a *Agent) SpawnRequest(req *pb.VmSpawnRequest) (*pb.VmSpawnResponse, error) {
    // 1. Dry-run query to all nodes
    // 2. Get first response
    // 3. Send actual spawn to that node
    // 4. Wait for response with container ID
}
```

### Handling Spawn on a Node
```go
// File: pkg/cluster/serf.go
func (a *Agent) Handler() {
    for event := range a.eventCh {
        if event.EventType() == serf.EventQuery {
            switch baseMessage.GetEvent() {
            case pb.ClusterEvent_SPAWN:
                response, err = a.handleSpawnRequest(&payload)
            }
        }
    }
}

func (a *Agent) handleSpawnRequest(payload *pb.VmSpawnRequest) ([]byte, error) {
    // 1. Check capacity (CPU & memory)
    // 2. Create container via containerd repo
    // 3. Return container ID
}
```

### Monitoring & Broadcasting
```go
// File: pkg/cluster/serf.go
func (a *Agent) monitorWorkloads() {
    ticker := time.NewTicker(WorkloadBroadcastPeriod) // 5 seconds
    for range ticker.C {
        // 1. Get all tasks from containerd
        // 2. Extract port mappings from labels
        // 3. Hash workload state
        // 4. If changed, broadcast via Serf UserEvent
        // 5. Register services with proxy
    }
}

func (a *Agent) monitorStateUpdates(respawn bool) {
    ticker := time.NewTicker(WorkloadBroadcastPeriod) // 5 seconds
    for range ticker.C {
        // 1. Check if any node hasn't updated in 15 seconds
        // 2. If respawn enabled, reschedule workloads
    }
}
```

## Data Structures

### Agent (Serf Cluster Agent)
```go
type Agent struct {
    eventCh         chan serf.Event           // Serf events
    serviceProxy    *ServiceProxy             // HTTP routing
    ctrRepo         *vcontainerd.Repo         // Containerd integration
    cfg             *serf.Config              // Serf config
    serf            *serf.Serf                // Serf gossip client
    baseURL         string                    // Domain suffix
    logger          *log.Logger               // Logging
    lastStateMu     sync.Mutex                // State lock
    lastStateSelf   *pb.NodeStateResponse     // This node's workloads
    lastStateUpdate map[string]SavedStatusUpdate  // Other nodes' state
    tmpStateUpdates map[string]*pb.NodeStateResponse  // Partial reassembly
    lastStateHash   string                    // Detect changes
    stateMu         sync.Mutex                // Hash lock

    // Prometheus metrics
    serfQueueDepth   prometheus.Gauge
    workloadCount    prometheus.Gauge
    broadcastSkipped prometheus.Counter
    stateChanges     prometheus.Counter
}
```

### ServiceProxy (HTTP Reverse Proxy)
```go
type ServiceProxy struct {
    mu                *sync.Mutex
    logger            *log.Logger
    tlsConfig         *TLSConfig
    proxiedPortMap    map[uint32]struct{}           // Active ports
    serviceIDPortMaps map[string]map[uint32]string  // containerID -> (port -> addr)
}

// Maps like: serviceIDPortMaps["uuid"] = {443: "192.168.127.15:8080"}
```

## Configuration Defaults

```go
// pkg/defaults/defaults.go
const (
    ContainerdNamespace = "vistara"
    ContainerdSocket = "/var/lib/hypercore/containerd.sock"
    HACFile = "hac.toml"
    StateRootDir = "/run/hypercore"
)

// Serf config (pkg/cluster/serf.go)
GossipInterval = 2 seconds
ProbeInterval = 5 seconds
SuspicionMult = 6
GossipNodes = 2
UserEventSizeLimit = 2048 bytes
MaxQueueDepth = 5000 events
WorkloadBroadcastPeriod = 5 seconds
```

## Scheduling Logic

```
SpawnRequest(cores=2, mem=512, image=alpine):
  1. Create dry-run request (VmSpawnRequest{dry_run: true})
  2. Broadcast query "hypercore_query" to all nodes
  3. Wait for first response (first-come-first-serve)
  4. If node responds, send actual spawn to ONLY that node
  5. Get container ID back
  6. Return container ID + URL

Capacity Check (per node):
  - vCPU: sum of running + requested <= max(numCPU, 225)
  - Memory: sum of running + requested <= MemAvailable
  - First to respond with spare capacity wins
```

## Network Architecture

```
External Request
  ↓
https://containerID.deployments.example.com:443
  ↓
Reverse Proxy (port 443 listener)
  ├─ Extract Host header: "containerID.deployments.example.com"
  ├─ Look up serviceIDPortMaps["containerID"][443]
  ├─ Get address: "192.168.127.15:8080"
  └─ Create reverse proxy to that address
  ↓
Internal Container
  192.168.127.15:8080 (runc container in network namespace)
  ├─ Subnet: 192.168.127.0/24
  ├─ CNI Plugins: ptp, firewall, tc-redirect-tap
  ├─ DNS: uses host /etc/resolv.conf
  └─ Running application
```

## Container Creation Steps

```go
CreateContainer(imageRef, ports, env, limits):
  1. Pull image from registry
  2. Create network namespace at /run/netns/{uuid}
  3. Create OCI spec with:
     - Image config
     - Environment variables
     - CPU limits (CFS quota)
     - Memory limits
     - Network namespace
     - Host resolv.conf
  4. Create containerd container
  5. Add CNI networks:
     - ptp (point-to-point)
     - firewall
     - tc-redirect-tap (for VM support)
  6. Create task (process)
  7. Start task
  8. Return container ID
```

## Monitoring & Metrics

### What Gets Monitored
- Serf event queue depth (every 5s)
- Number of running workloads (every 5s)
- Container state (running/stopped)
- Node alive/dead status (15s timeout)
- State changes (hash comparison)

### Actions on Events
- Container stopped? → Respawn if enabled
- Node dead? → Reschedule workloads if enabled
- State changed? → Broadcast to cluster
- Queue depth high? → Skip broadcast to prevent overload

### Prometheus Metrics
```
hypercore_serf_queue_depth         # Current queue depth
hypercore_workload_count           # Running workloads on node
hypercore_broadcast_skipped_total  # Failed broadcasts (due to queue)
hypercore_state_changes_total      # Number of state changes
```

## Important Constants

| Constant | Value | Purpose |
|----------|-------|---------|
| QueryName | "hypercore_query" | Serf query event name |
| SpawnRequestLabel | "hypercore-request-payload" | Container label for spawn request |
| StateBroadcastEvent | "hypercore_state_broadcast" | Serf user event name |
| WorkloadBroadcastPeriod | 5 seconds | State broadcast frequency |
| MaxQueueDepth | 5000 | Max Serf queue depth |
| GossipInterval | 2 seconds | Serf gossip frequency |
| ProbeInterval | 5 seconds | Serf probe frequency |
| FailureTimeout | 15 seconds | Mark node dead after 3 missed broadcasts |

## IBRL Integration Points - Summary

| Module | File Location | Integration Type | Priority |
|--------|---------------|------------------|----------|
| **Beacon** | `pkg/beacon/` (NEW) | Node attestation + discovery | Phase 1 |
| **Policy** | `pkg/policy/` (NEW) | Scheduling + permission enforcement | Phase 2 |
| **Proof** | `pkg/proof/` (NEW) | Computation proof + attestation | Phase 3 |

### Key Integration Locations for IBRL
1. **Beacon Node Registration**: Extend `NodeStateResponse` proto
2. **Policy-based Scheduling**: Wrap `SpawnRequest()` logic
3. **Proof Generation**: Hook in `monitorWorkloads()` 
4. **State Attestation**: Add to broadcast messages
5. **Metrics**: Register IBRL-specific Prometheus metrics

