# IBRL End-to-End Testing Guide

This guide demonstrates how to test the complete IBRL integration in Hypercore, from beacon metrics collection to policy-based workload routing.

## Prerequisites

- Hypercore binary built with IBRL integration (`make build`)
- At least 2-3 machines/VMs for multi-node testing
- Containerd installed and running on each node
- Network connectivity between nodes

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    IBRL-Enabled Hypercore Cluster           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Node 1                  Node 2                  Node 3     │
│  ┌──────────┐           ┌──────────┐           ┌──────────┐│
│  │ Beacon   │           │ Beacon   │           │ Beacon   ││
│  │ Metrics  │           │ Metrics  │           │ Metrics  ││
│  ├──────────┤           ├──────────┤           ├──────────┤│
│  │ Latency: │           │ Latency: │           │ Latency: ││
│  │  10ms    │           │  50ms    │           │  100ms   ││
│  │ Price:   │           │ Price:   │           │ Price:   ││
│  │  $0.02   │           │  $0.01   │           │  $0.005  ││
│  │ Reputation: │        │ Reputation: │        │ Reputation: ││
│  │  1.0     │           │  0.8     │           │  0.9     ││
│  └──────────┘           └──────────┘           └──────────┘│
│       ▲                      ▲                      ▲       │
│       │                      │                      │       │
│       └──────────────────────┴──────────────────────┘       │
│                     Serf Gossip Network                     │
│                                                             │
│                    Policy Engine                            │
│            ┌────────────────────────────────┐              │
│            │  low-latency.json → Node 1     │              │
│            │  cost-optimized.json → Node 3  │              │
│            │  balanced.json → Node 2        │              │
│            └────────────────────────────────┘              │
└─────────────────────────────────────────────────────────────┘
```

## Phase 1: Single Node Setup (Baseline)

### Step 1: Build Hypercore

```bash
cd /home/user/hypercore
make build

# Verify build
./bin/hypercore --help
```

### Step 2: Start Single Node

```bash
# Terminal 1: Start cluster node
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  --cluster-base-url node1.local \
  --grpc-bind-addr 0.0.0.0:8000 \
  --http-bind-addr 0.0.0.0:8001
```

### Step 3: Verify Beacon Metrics

```bash
# Terminal 2: Check cluster metrics
./bin/hypercore cluster metrics

# Expected output:
# IBRL Cluster Metrics:
# ====================
#
# Node: 5f3a2b1c-...
#   Beacon ID:       a1b2c3d4e5f6g7h8
#   Latency:         0.00 ms
#   Jitter:          0.00 ms
#   Packet Loss:     0.00%
#   Queue Depth:     0
#   Price/GB:        $0.0100
#   Reputation:      1.0
#   Capabilities:    [container vm]
#   Workloads:       0
```

### Step 4: Test Basic Spawn (No Policy)

```bash
# Spawn a workload without policy
./bin/hypercore cluster spawn \
  --cpu 1 \
  --mem 512 \
  --image-ref docker.io/library/nginx:latest \
  --ports 8080:80

# Check workload list
./bin/hypercore cluster list
```

## Phase 2: Multi-Node Setup

### Step 1: Start First Node (Master)

```bash
# Node 1 (192.168.1.10)
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  --cluster-base-url node1.example.com \
  --grpc-bind-addr 0.0.0.0:8000 \
  --http-bind-addr 0.0.0.0:8001 \
  --cluster-policy examples/policies/balanced.json

# Note the node IP for joining
```

### Step 2: Start Additional Nodes

```bash
# Node 2 (192.168.1.11)
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  --cluster-base-url node2.example.com \
  --grpc-bind-addr 0.0.0.0:8000 \
  --http-bind-addr 0.0.0.0:8001 \
  192.168.1.10:7946  # Join node 1

# Node 3 (192.168.1.12)
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  --cluster-base-url node3.example.com \
  --grpc-bind-addr 0.0.0.0:8000 \
  --http-bind-addr 0.0.0.0:8001 \
  192.168.1.10:7946  # Join node 1
```

### Step 3: Verify Cluster Formation

```bash
# On any node
./bin/hypercore cluster metrics

# Expected output:
# IBRL Cluster Metrics:
# ====================
#
# Node: node1-uuid
#   Beacon ID:       node1-beacon-id
#   Latency:         0.00 ms
#   ...
#   Workloads:       0
#
# Node: node2-uuid
#   Beacon ID:       node2-beacon-id
#   Latency:         5.23 ms
#   ...
#   Workloads:       0
#
# Node: node3-uuid
#   Beacon ID:       node3-beacon-id
#   Latency:         8.45 ms
#   ...
#   Workloads:       0
```

## Phase 3: Policy-Based Routing Tests

### Test 1: Low-Latency Policy

This policy should select the node with the lowest latency.

```bash
# Spawn with low-latency policy
./bin/hypercore cluster spawn \
  --cpu 1 \
  --mem 512 \
  --image-ref docker.io/library/nginx:latest \
  --ports 9001:80 \
  --policy examples/policies/low-latency.json

# Check logs to see which node was selected
# Look for: "successfully spawned VM on policy-selected node"

# Verify workload placement
./bin/hypercore cluster list
```

**Expected Behavior:**
- Policy engine evaluates all nodes based on latency
- Selects node with lowest latency_ms value
- Logs show: `policy-based node selection completed` with selected node
- Workload spawns on the lowest-latency node

### Test 2: Cost-Optimized Policy

This policy should select the cheapest node that meets quality requirements.

```bash
# Spawn with cost-optimized policy
./bin/hypercore cluster spawn \
  --cpu 1 \
  --mem 512 \
  --image-ref docker.io/library/redis:latest \
  --ports 9002:6379 \
  --policy examples/policies/cost-optimized.json

# Check which node was selected
./bin/hypercore cluster list
```

**Expected Behavior:**
- Policy engine ranks nodes by price (price_per_gb)
- Filters out nodes exceeding max_price_per_gb (0.05)
- Selects cheapest qualifying node
- Falls back to next-cheapest if first choice fails

### Test 3: High-Trust Policy

This policy should only use nodes with high reputation scores.

```bash
# Spawn with high-trust policy
./bin/hypercore cluster spawn \
  --cpu 2 \
  --mem 1024 \
  --image-ref docker.io/library/postgres:latest \
  --ports 9003:5432 \
  --policy examples/policies/high-trust.json
```

**Expected Behavior:**
- Only nodes with reputation_score >= 0.9 are candidates
- If no nodes meet criteria, spawn fails with policy violation error
- Selects highest-reputation node among candidates

### Test 4: Balanced Policy

This policy balances latency, price, reputation, and queue depth.

```bash
# Spawn with balanced policy
./bin/hypercore cluster spawn \
  --cpu 1 \
  --mem 512 \
  --image-ref docker.io/library/busybox:latest \
  --ports 9004:8080 \
  --policy examples/policies/balanced.json
```

**Expected Behavior:**
- Calculates weighted score for each node
- Score formula: `(0.25 * latency_score) + (0.25 * price_score) + (0.25 * reputation_score) + (0.25 * queue_score)`
- Selects node with highest total score

### Test 5: Policy Violation Handling

Test what happens when no nodes meet policy constraints.

```bash
# Create a very restrictive policy
cat > /tmp/ultra-strict.json <<EOF
{
  "name": "ultra-strict",
  "description": "Impossible constraints",
  "mode": "enforce",
  "rules": {
    "max_latency_ms": 1,
    "max_price_per_gb": 0.001,
    "min_reputation_score": 0.99
  },
  "scoring": {
    "latency_weight": 1.0,
    "price_weight": 0.0,
    "reputation_weight": 0.0,
    "queue_weight": 0.0
  }
}
EOF

# Try to spawn with impossible constraints
./bin/hypercore cluster spawn \
  --cpu 1 \
  --mem 512 \
  --image-ref docker.io/library/alpine:latest \
  --policy /tmp/ultra-strict.json
```

**Expected Behavior:**
- All nodes fail constraint checks
- Error: `no nodes match policy constraints`
- Logs show constraint violations with details

## Phase 4: Observability and Monitoring

### Prometheus Metrics

Hypercore exposes IBRL metrics on the HTTP endpoint:

```bash
# Check Prometheus metrics
curl http://localhost:8001/metrics | grep ibrl

# Expected metrics:
# hypercore_ibrl_beacon_connected{} 1
# hypercore_ibrl_policy_evaluations_total{} 15
# hypercore_ibrl_policy_violations_total{} 2
```

### Log Analysis

```bash
# View policy evaluation logs
sudo journalctl -u hypercore -f | grep policy

# Expected log entries:
# level=info msg="loaded policy" name=balanced mode=enforce
# level=info msg="policy-based node selection completed" policy=balanced selected=3 top_node=node1-uuid
# level=info msg="attempting to spawn on policy-selected node" node=node1-uuid priority=1 total=3
# level=info msg="successfully spawned VM on policy-selected node" node=node1-uuid
```

### Viewing Cluster State

```bash
# Full cluster state
./bin/hypercore cluster list

# Beacon metrics for all nodes
./bin/hypercore cluster metrics

# Logs for specific workload
./bin/hypercore cluster logs --id <workload-id>
```

## Phase 5: Advanced Scenarios

### Scenario 1: Dynamic Policy Updates

Test changing policy at runtime (future enhancement - currently requires restart):

```bash
# Stop node with Ctrl+C
# Restart with new policy
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  --cluster-policy examples/policies/low-latency.json \
  192.168.1.10:7946
```

### Scenario 2: Failover Testing

Simulate node failure and observe policy-based failover:

```bash
# 1. Spawn workload on node 1
./bin/hypercore cluster spawn --policy examples/policies/balanced.json ...

# 2. Kill node 1
sudo pkill -f hypercore

# 3. Spawn another workload - should automatically select node 2 or 3
./bin/hypercore cluster spawn --policy examples/policies/balanced.json ...

# Check logs - should show node 1 filtered out, next-best selected
```

### Scenario 3: Load Balancing

Spawn multiple workloads and observe distribution:

```bash
# Spawn 10 workloads with balanced policy
for i in {1..10}; do
  ./bin/hypercore cluster spawn \
    --cpu 1 \
    --mem 256 \
    --image-ref docker.io/library/nginx:latest \
    --ports $((9000+i)):80 \
    --policy examples/policies/balanced.json
  sleep 2
done

# Check distribution
./bin/hypercore cluster metrics
```

**Expected Behavior:**
- Workloads distributed based on queue_depth in scoring
- Nodes with higher queue depth get lower scores
- Achieves natural load balancing

## Phase 6: Troubleshooting

### Issue 1: Policy File Not Found

```bash
# Error: failed to load policy file: no such file or directory

# Solution: Use absolute path
sudo ./bin/hypercore cluster \
  --cluster-policy /home/user/hypercore/examples/policies/balanced.json \
  ...
```

### Issue 2: All Nodes Fail Policy Constraints

```bash
# Error: no nodes match policy constraints

# Solution 1: Check node metrics
./bin/hypercore cluster metrics

# Solution 2: Relax policy constraints or use permissive mode
./bin/hypercore cluster spawn --policy examples/policies/permissive.json ...
```

### Issue 3: Beacon Metadata Missing

```bash
# Symptom: Node shows "Beacon: Not available"

# Cause: Node just joined, hasn't broadcast state yet
# Solution: Wait 5-10 seconds for first broadcast, then check again
sleep 10
./bin/hypercore cluster metrics
```

## Success Criteria

✅ **Phase 1 Complete:**
- Single node starts successfully
- Beacon metrics visible
- Basic spawn works

✅ **Phase 2 Complete:**
- Multi-node cluster forms
- All nodes show in metrics
- Serf gossip working

✅ **Phase 3 Complete:**
- Policy-based routing works
- Different policies select different nodes
- Policy violations handled gracefully

✅ **Phase 4 Complete:**
- Prometheus metrics exposed
- Logs show policy decisions
- Cluster state observable

✅ **Phase 5 Complete:**
- Failover works
- Load balancing observed
- Policy updates successful

## Quick Reference Commands

```bash
# Start cluster node with policy
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  --cluster-policy examples/policies/balanced.json

# Join existing cluster
sudo ./bin/hypercore cluster \
  --cluster-bind-addr 0.0.0.0:7946 \
  192.168.1.10:7946

# View cluster metrics
./bin/hypercore cluster metrics

# Spawn with policy
./bin/hypercore cluster spawn \
  --image-ref docker.io/library/nginx:latest \
  --policy examples/policies/low-latency.json

# List workloads
./bin/hypercore cluster list

# Stop workload
./bin/hypercore cluster stop --id <workload-id>

# View logs
./bin/hypercore cluster logs --id <workload-id>
```

## Next Steps

After completing this guide, you can:

1. **Customize Policies**: Create your own policy files based on your workload requirements
2. **Monitor Performance**: Set up Grafana dashboards for IBRL metrics
3. **Test Phase 3**: When proof-of-delivery is implemented, verify workload execution proofs
4. **Production Deployment**: Deploy IBRL-enabled Hypercore cluster in production

---

**Need Help?** Check the logs:
```bash
sudo journalctl -u hypercore -f | grep -E "policy|beacon|ibrl"
```
