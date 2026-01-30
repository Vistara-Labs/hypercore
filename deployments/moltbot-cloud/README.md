# Clawdbot Cloud Deployment for Hypercore

This directory contains Docker images and configuration for deploying Clawdbot on Hypercore microVMs.

## Images

### Dockerfile.npm (Recommended)
Builds from the published npm package. Fast and reliable.

```bash
cd deployments/moltbot-cloud
docker build -f Dockerfile.npm -t clawdbot-vm:npm .
```

### Dockerfile (From Source)
Builds from Clawdbot source code. Use this for custom builds.

**Build from Clawdbot source repo:**
```bash
# In the clawdbot source directory
cp /path/to/hypercore/deployments/moltbot-cloud/entrypoint.sh ./entrypoint.sh
docker build -f /path/to/hypercore/deployments/moltbot-cloud/Dockerfile -t clawdbot-vm:source .
```

## Running

### Docker (Standalone)
```bash
docker run -d \
  --name clawdbot \
  -p 18789:18789 \
  -e ANTHROPIC_API_KEY="your-api-key" \
  -e CLAWDBOT_GATEWAY_TOKEN="your-token"  # optional, auto-generated if not set \
  clawdbot-vm:npm
```

### Hypercore MicroVM
```bash
# Push to registry
docker tag clawdbot-vm:npm registry.your.domain/clawdbot:latest
docker push registry.your.domain/clawdbot:latest

# Deploy via Hypercore
hypercore cluster spawn \
  --grpc-bind-addr "$NODE_IP:8000" \
  --ports 443:18789 \
  --image-ref registry.your.domain/clawdbot:latest
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Your Anthropic API key |
| `CLAWDBOT_GATEWAY_TOKEN` | No | Gateway auth token (auto-generated if not set) |
| `CLAWDBOT_PORT` | No | Gateway port (default: 18789) |
| `CLAWDBOT_MODEL` | No | Default model (default: `anthropic/claude-sonnet-4-20250514`) |

## Health Check

The container includes a health check that verifies the gateway is serving the Control UI:
- Interval: 10s
- Start period: 30s
- Endpoint: `http://localhost:18789/`

## Connecting

Once running, connect to the gateway:
- **Control UI:** `http://your-host:18789/`
- **WebSocket:** `ws://your-host:18789/`

Pass the token (shown in container logs) via `connect.params.auth.token`.

## Hypercore Requirements

Full Hypercore deployment requires:
- **KVM support** (`/dev/kvm` available)
- **dmsetup** for containerd snapshotter
- Static public IP with ports exposed

Without KVM (e.g., on a VPS that's already a VM), use Docker standalone mode.
