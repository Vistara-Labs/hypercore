# Serf Queue Depth Fix

## Problem
The Serf event queue was continuously growing (2130 -> 2190) due to:
1. Unconditional state broadcasting every 5 seconds
2. No state change detection
3. No queue depth throttling
4. Aggressive Serf configuration

## Fixes Applied

### 1. State Change Detection
- Added `hashWorkloadState()` function to create consistent hash of workload state
- Added `lastStateHash` field to track previous state
- Only broadcast when state actually changes
- Added `stateMu` mutex for thread-safe state tracking

### 2. Queue Depth Throttling
- Added `MaxQueueDepth = 1000` constant
- Check `event_queue_depth` from `a.serf.Stats()` before broadcasting
- Skip broadcast if queue depth exceeds limit
- Parse string value to integer using `strconv.Atoi`

### 3. Increased Broadcast Period
- Changed `WorkloadBroadcastPeriod` from 5s to 30s
- Reduces broadcast frequency by 6x

### 4. Conservative Serf Configuration
- Increased `UserEventSizeLimit` to 2048
- Set `GossipInterval` to 2s (conservative)
- Set `ProbeInterval` to 5s (conservative)
- Set `SuspicionMult` to 6 (increased stability)
- Set `GossipNodes` to 2 (reduced load)

## Code Changes in `pkg/cluster/serf.go`

```go
// Added imports
"crypto/sha256"
"encoding/hex"
"sort"

// Added constants
WorkloadBroadcastPeriod = time.Second * 30 // Increased from 5s
MaxQueueDepth = 1000

// Added fields to Agent struct
lastStateHash string
stateMu       sync.Mutex

// Added function
func (a *Agent) hashWorkloadState(state *pb.NodeStateResponse) string {
    // Creates deterministic hash of workload state
}

// Updated monitorWorkloads()
// 1. Calculate currentHash = a.hashWorkloadState(&resp)
// 2. Check stateChanged := currentHash != a.lastStateHash
// 3. Only broadcast if stateChanged
// 4. Check queue depth before broadcasting
// 5. Skip if queue depth > MaxQueueDepth
```

## Expected Results
- Queue depth should stabilize and not continuously increase
- Reduced network traffic from fewer broadcasts
- Better system stability under load
- Proper state synchronization maintained

## Deployment
Build and deploy to GCP VMs. Monitor queue depth with:
```bash
sudo journalctl -f -u hypercore-cluster.service --no-pager | grep "queue depth"
``` 