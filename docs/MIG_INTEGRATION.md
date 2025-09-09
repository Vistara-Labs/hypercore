# 🚀 MIG Integration with Hypercore

This document explains how to use NVIDIA MIG (Multi-Instance GPU) with Hypercore's GPU sub-scheduler for efficient GPU resource sharing.

## 🎯 Overview

The MIG integration provides:
- **NVML-based MIG discovery** (no `nvidia-smi` parsing)
- **Automatic MIG device reservation** with lockfile mechanism
- **Profile-based allocation** (1g.10gb, 2g.20gb, etc.)
- **Fallback to whole GPU** when MIG unavailable
- **Comprehensive monitoring** with Prometheus metrics

## 🏗️ Architecture

```
Hypercore Runner
├── MIG Discovery (NVML)
├── Device Reservation (Lockfile)
├── CUDA_VISIBLE_DEVICES (MIG UUID)
└── Model Execution (with quotas)
```

## 🔧 Components

### 1. NVML Manager (`internal/mig/nvmlmgr.go`)
- **Initializes NVML** library
- **Lists all physical GPUs** and their MIG devices
- **Returns MIG UUID**, parent GPU index, and memory size
- **Provides reservation helper** with lockfile mechanism

### 2. Reservation System (`internal/mig/reservation.go`)
- **Profile parsing** (1g.10gb, 2g.20gb, custom profiles)
- **Environment-based configuration** (HC_MIG_PROFILE)
- **TTL-based lock cleanup** (default 5 minutes)
- **Available profiles listing**

### 3. Runner Integration (`cmd/runner/main.go`)
- **Automatic MIG reservation** on startup
- **Fallback to whole GPU** if MIG fails
- **Environment variable support** (HC_MIG_PROFILE)
- **Clean resource release** on shutdown

## 📋 Configuration

### Environment Variables

```bash
# MIG Profile (required for MIG mode)
export HC_MIG_PROFILE="1g.10gb"  # or "2g.20gb", "4g.40gb", "custom.10240"

# Optional: TTL for MIG locks (default: 5m)
export HC_MIG_TTL="10m"

# Standard GPU configuration
export HYPERCORE_VRAM_LIMIT_BYTES="10737418240"  # 10GB
export HYPERCORE_CPU_MEM_MB="2048"
export CUDA_VISIBLE_DEVICES="0"  # Will be overridden with MIG UUID
```

### Supported MIG Profiles

| Profile | Memory | Use Case |
|---------|--------|----------|
| `1g.5gb` | ~5GB | Small models, inference |
| `1g.10gb` | ~10GB | Medium models, fine-tuning |
| `2g.20gb` | ~20GB | Large models, training |
| `4g.40gb` | ~40GB | Very large models |
| `custom.N` | N MB | Custom memory requirements |

## 🚀 Usage Examples

### Basic MIG Usage

```bash
# Start runner with MIG profile
export HC_MIG_PROFILE="1g.10gb"
./bin/runner --target python3 --args /opt/models/server.py
```

### Multiple Runners with Different Profiles

```bash
# Terminal 1: Small model
export HC_MIG_PROFILE="1g.5gb"
export MODEL_ID="small-model"
./bin/runner --target python3 --args /opt/models/small.py

# Terminal 2: Large model  
export HC_MIG_PROFILE="2g.20gb"
export MODEL_ID="large-model"
./bin/runner --target python3 --args /opt/models/large.py
```

### Custom Memory Profile

```bash
# Custom 8GB profile
export HC_MIG_PROFILE="custom.8192"
./bin/runner --target python3 --args /opt/models/custom.py
```

### Fallback to Whole GPU

```bash
# If MIG fails, falls back to whole GPU
export HC_MIG_PROFILE="1g.10gb"  # Will try MIG first
export CUDA_VISIBLE_DEVICES="0"  # Fallback device
./bin/runner --target python3 --args /opt/models/server.py
```

## 🔍 Monitoring

### Prometheus Metrics

The runner exposes these metrics for MIG monitoring:

```prometheus
# MIG device information (via labels)
hypercore_gpu_quota_exceeded_total{instance="runner-1", mig_uuid="MIG-GPU-..."}

# Model performance
hypercore_infer_latency_seconds_bucket{le="0.1", instance="runner-1"}
hypercore_model_restarts_total{instance="runner-1"}

# Resource utilization
hypercore_gpu_vram_utilization_percent{instance="runner-1"}
hypercore_gpu_cpu_mem_utilization_percent{instance="runner-1"}
```

### Grafana Dashboard

Import the provided dashboard (`monitoring/grafana-hypercore-gpu.json`) to visualize:

- **p95 latency** from histogram metrics
- **Quota exceeded rate** (MIG violations)
- **Model restarts** (recovery events)
- **Request throughput** (reqs/sec)
- **GPU utilization** (if DCGM available)

## 🛠️ Development

### Building with MIG Support

```bash
# Build with CGO enabled for NVML
make build-gpu

# Or use the GPU build script
./scripts/build-gpu.sh
```

### Testing MIG Discovery

```go
package main

import (
    "fmt"
    "log"
    "vistara-node/internal/mig"
)

func main() {
    // List all available devices
    devices, err := mig.ListAll()
    if err != nil {
        log.Fatal(err)
    }
    
    for _, dev := range devices {
        fmt.Printf("GPU %d: UUID=%s, Memory=%dB, MIG=%v, Profile=%s\n",
            dev.GPUIndex, dev.UUID, dev.MemoryB, dev.IsMIG, mig.GuessProfile(dev.MemoryB))
    }
    
    // Reserve a MIG device
    dev, release, err := mig.ReserveByProfile("1g.10gb", 5*time.Minute)
    if err != nil {
        log.Fatal(err)
    }
    defer release()
    
    fmt.Printf("Reserved: %s\n", dev.UUID)
}
```

### Testing Reservation

```bash
# Test MIG reservation
export HC_MIG_PROFILE="1g.10gb"
./bin/runner --target echo --args "MIG device reserved"

# Check lock files
ls -la /var/run/hypercore/mig/
```

## 🔧 Troubleshooting

### Common Issues

1. **NVML library not found**
   ```bash
   # Install NVML development package
   sudo apt-get install nvidia-ml-dev
   # or
   sudo yum install nvidia-ml-devel
   ```

2. **MIG mode not enabled**
   ```bash
   # Check MIG status
   nvidia-smi -mig -l
   
   # Enable MIG (requires reboot)
   sudo nvidia-smi -mig 1
   ```

3. **Permission denied for lock files**
   ```bash
   # Create lock directory with proper permissions
   sudo mkdir -p /var/run/hypercore/mig
   sudo chmod 755 /var/run/hypercore/mig
   ```

4. **No MIG devices available**
   ```bash
   # Check available MIG instances
   nvidia-smi -L
   
   # Create MIG instances if needed
   sudo nvidia-smi -mig -cgi 1g.10gb -C
   ```

### Debug Mode

```bash
# Enable debug logging
export HC_DEBUG=1
export HC_MIG_PROFILE="1g.10gb"
./bin/runner --target python3 --args /opt/models/server.py
```

## 📊 Performance Tips

### For Better Latency

```bash
# Enable prefetch for better cold-start performance
export HYPERCORE_PREFETCH=1

# Use smaller VRAM limits to trigger quota events for testing
export HYPERCORE_VRAM_LIMIT_BYTES=$((2*1024*1024*1024))  # 2GB
```

### For Production

```bash
# Use appropriate MIG profiles for your models
export HC_MIG_PROFILE="1g.10gb"  # For 7B models
export HC_MIG_PROFILE="2g.20gb"  # For 13B models
export HC_MIG_PROFILE="4g.40gb"  # For 70B models

# Set reasonable TTL for locks
export HC_MIG_TTL="10m"  # 10 minutes
```

## 🔄 Migration from nvidia-smi

If you're migrating from `nvidia-smi` parsing:

1. **Remove nvidia-smi calls** from your code
2. **Add HC_MIG_PROFILE** environment variable
3. **Update CUDA_VISIBLE_DEVICES** to use MIG UUIDs
4. **Test with fallback** to whole GPU

The NVML approach is more robust and doesn't require parsing text output.

## 📚 References

- [NVIDIA MIG Documentation](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/)
- [NVML API Reference](https://docs.nvidia.com/deploy/nvml-api/)
- [Hypercore GPU Integration](GPU_INTEGRATION.md)
- [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)