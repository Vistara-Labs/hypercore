# Claude Agent SDK Integration with Hypercore

**Production-ready integration for 10,000+ concurrent users**

## Overview

This integration enables running Claude Agent SDK instances as isolated microVM sandboxes within Hypercore, providing secure, scalable AI agent hosting for multi-tenant applications.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Requests (10k+)                     │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Hypercore Agent Manager API                    │
│         (Load Balancing, Rate Limiting, Auth)              │
└────────────────────────┬────────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         │               │               │
         ▼               ▼               ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Claude Agent │  │ Claude Agent │  │ Claude Agent │
│  (MicroVM 1) │  │  (MicroVM 2) │  │  (MicroVM N) │
│              │  │              │  │              │
│ - FastAPI    │  │ - FastAPI    │  │ - FastAPI    │
│ - Anthropic  │  │ - Anthropic  │  │ - Anthropic  │
│ - Monitoring │  │ - Monitoring │  │ - Monitoring │
└──────────────┘  └──────────────┘  └──────────────┘
         │               │               │
         └───────────────┼───────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Autoscaler Service                       │
│           (Prometheus Metrics → Scaling Decisions)         │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

### 🔒 **Security & Isolation**
- **MicroVM Isolation**: Each agent runs in a separate Firecracker microVM
- **Network Isolation**: TAP devices provide network-level separation
- **Non-root Containers**: All processes run as unprivileged users
- **Resource Quotas**: CPU, memory, and GPU limits enforced via cgroups

### ⚡ **Performance & Scalability**
- **10k+ Concurrent Users**: Designed and tested for high concurrency
- **Auto-scaling**: Automatic agent spawning based on load metrics
- **Multi-process Workers**: Uvicorn workers for CPU-bound tasks
- **Connection Pooling**: Efficient resource utilization
- **GPU Support**: Optional GPU allocation via MIG slices

### 📊 **Observability**
- **Prometheus Metrics**: Request rates, latency, tokens, errors
- **Grafana Dashboards**: Real-time monitoring and alerting
- **Structured Logging**: JSON logs for easy parsing
- **Health Checks**: Kubernetes-style readiness/liveness probes
- **Distributed Tracing**: OpenTelemetry support

### 💰 **Cost Optimization**
- **Dynamic Scaling**: Scale down during low traffic
- **Resource Sharing**: Efficient GPU utilization with quotas
- **Token Usage Tracking**: Monitor API costs per user
- **Warm Agent Pools**: Reduce cold start latency

## Installation

### Prerequisites

- Docker (for building images)
- Go 1.21+ (for integration services)
- Python 3.11+ (for agent runtime)
- Hypercore cluster (deployed and accessible)
- Anthropic API key

### Quick Start

1. **Clone and configure**:
```bash
cd /path/to/hypercore
cp deployments/claude-agent/.env.example deployments/claude-agent/.env
# Edit .env with your Anthropic API key
```

2. **Deploy everything**:
```bash
export ANTHROPIC_API_KEY="sk-ant-api03-..."
./scripts/deploy-claude-agents.sh
```

3. **Verify deployment**:
```bash
# Check services
systemctl status hypercore-agent-manager
systemctl status agent-autoscaler

# Check metrics
curl http://localhost:8080/health
curl http://localhost:8080/v1/agents/stats
```

## Usage

### Spawn an Agent

```bash
curl -X POST http://localhost:8080/v1/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "anthropic_api_key": "sk-ant-api03-...",
    "cores": 4,
    "memory": 8192,
    "max_concurrent": 100,
    "system_prompt": "You are a helpful AI assistant."
  }'
```

**Response**:
```json
{
  "agent_id": "06d0f10a-a6c6-45ae-8f23-770b96851bc3",
  "url": "06d0f10a.deployments.vistara.dev",
  "metrics_url": "https://06d0f10a.deployments.vistara.dev/metrics",
  "status": "running",
  "created_at": "2025-10-19T10:30:00Z",
  "user_id": "user-123",
  "cores": 4,
  "memory": 8192
}
```

### Chat with Agent

```bash
curl -X POST https://06d0f10a.deployments.vistara.dev/v1/agent/chat \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Explain quantum computing",
    "user_id": "user-123",
    "max_tokens": 4096,
    "stream": false
  }'
```

**Response**:
```json
{
  "session_id": "session-abc123",
  "content": "Quantum computing is...",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 150,
    "total_tokens": 160
  },
  "status": "completed"
}
```

### List User Agents

```bash
curl http://localhost:8080/v1/agents/list?user_id=user-123
```

### Delete Agent

```bash
curl -X DELETE http://localhost:8080/v1/agents/delete?agent_id=06d0f10a-a6c6-45ae-8f23-770b96851bc3
```

## Configuration

### Agent Manager Configuration

Environment variables for `hypercore-agent-manager`:

| Variable | Default | Description |
|----------|---------|-------------|
| `HYPERCORE_ADDR` | `localhost:8000` | Hypercore API address |
| `REGISTRY_URL` | `registry.vistara.dev` | Container registry |
| `MAX_AGENTS_PER_USER` | `50` | Maximum agents per user |
| `LISTEN_ADDR` | `:8080` | API listen address |

### Autoscaler Configuration

Environment variables for `agent-autoscaler`:

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMETHEUS_URL` | `http://localhost:9090` | Prometheus API endpoint |
| `MIN_AGENTS` | `10` | Minimum agents always running |
| `MAX_AGENTS` | `1000` | Maximum agents (10k users = ~100 sessions/agent) |
| `TARGET_SESSIONS_PER_AGENT` | `100` | Target concurrency per agent |
| `SCALE_UP_THRESHOLD` | `0.80` | Scale up at 80% capacity |
| `SCALE_DOWN_THRESHOLD` | `0.30` | Scale down at 30% capacity |

### Agent Container Configuration

See [.env.example](../deployments/claude-agent/.env.example) for full configuration options.

## Monitoring

### Prometheus Metrics

**Agent Metrics**:
- `agent_active_sessions` - Current active sessions
- `agent_requests_total{endpoint, status}` - Total requests by endpoint and status
- `agent_request_duration_seconds` - Request latency histogram
- `agent_token_usage_total{type}` - Token usage (input/output)
- `agent_errors_total{error_type}` - Errors by type

**Hypercore Metrics**:
- `hypercore_agent_spawn_total{status}` - Agent spawn operations
- `hypercore_agent_active_count` - Active agent containers
- `hypercore_agent_spawn_duration_seconds` - Spawn duration

### Grafana Dashboard

Import the dashboard from `monitoring/grafana-agent-dashboard.json`:

1. Open Grafana → Dashboards → Import
2. Upload `grafana-agent-dashboard.json`
3. Select Prometheus datasource
4. Click Import

**Dashboard Panels**:
- Active sessions over time
- Request rate (req/sec)
- Latency percentiles (p95, p99)
- Token usage (input/output)
- Error rate by type
- Agent spawn rate and duration
- Success rate percentage
- Resource usage (CPU/Memory)

### Alerts

Prometheus alerting rules are configured in `monitoring/prometheus-agent-rules.yml`:

- **HighAgentErrorRate**: Error rate > 0.1/sec for 2 minutes
- **HighAgentLatency**: p95 latency > 30s for 5 minutes
- **AgentSpawnFailures**: Spawn failure rate > 0.05/sec
- **HighAgentSessionCount**: Active sessions > 5000 (capacity warning)
- **CriticalAgentCapacity**: Active sessions > 9000 (90% capacity)
- **LowAgentSuccessRate**: Success rate < 95% for 5 minutes

## Scaling for 10k Users

### Capacity Planning

**Configuration for 10,000 concurrent users**:
- **Sessions per agent**: 100
- **Required agents**: 100 (at full capacity)
- **Min agents**: 10 (warm pool)
- **Max agents**: 1000 (with buffer)
- **Scale-up trigger**: 80% capacity (8,000 sessions)
- **Scale-down trigger**: 30% capacity (3,000 sessions)

### Resource Requirements

**Per Agent**:
- CPU: 4 cores
- Memory: 8GB RAM
- Network: 1Gbps
- Storage: 10GB

**For 100 Agents (full load)**:
- Total CPU: 400 cores
- Total Memory: 800GB
- Estimated cost: ~$500-1000/month (excluding API tokens)

**API Token Costs** (estimated):
- Avg tokens per request: 1,000 (500 input + 500 output)
- Cost per request: ~$0.008 (using Claude 3.5 Sonnet)
- 10k users × 10 requests/day: $800/day = $24k/month

### Performance Benchmarks

**Expected Performance**:
- Request latency p95: < 2 seconds (excluding Claude API time)
- Request latency p99: < 5 seconds
- Spawn time p95: < 10 seconds
- Success rate: > 99%
- Max throughput: 10,000 concurrent requests

## Best Practices

### Security

1. **API Key Management**:
   - Use separate API keys per user (pass via spawn request)
   - Rotate keys regularly
   - Use secrets management (Vault, AWS Secrets Manager)

2. **Network Security**:
   - Deploy behind load balancer with TLS
   - Use rate limiting at API gateway
   - Implement authentication/authorization

3. **Resource Limits**:
   - Set memory limits to prevent OOM
   - Use cgroup quotas for CPU/GPU
   - Monitor and alert on quota violations

### Reliability

1. **High Availability**:
   - Run multiple agent manager instances
   - Use Redis for shared session state
   - Deploy across multiple availability zones

2. **Graceful Degradation**:
   - Implement circuit breakers
   - Queue requests during high load
   - Return 503 when at capacity

3. **Disaster Recovery**:
   - Regular backups of configuration
   - Documented rollback procedures
   - Test failover scenarios

### Cost Optimization

1. **Token Management**:
   - Implement max token limits per user
   - Cache common responses
   - Use cheaper models for simple tasks

2. **Resource Optimization**:
   - Scale down aggressively during low traffic
   - Use spot instances where possible
   - Monitor idle agents and terminate

3. **Multi-tenancy**:
   - Share agents across low-traffic users
   - Implement session multiplexing
   - Use request queuing

## Troubleshooting

### Agent Won't Spawn

**Problem**: `curl http://localhost:8080/v1/agents/spawn` returns error

**Solutions**:
1. Check hypercore connectivity:
   ```bash
   curl http://<hypercore-addr>/health
   ```

2. Verify container image exists:
   ```bash
   docker images | grep claude-agent
   ```

3. Check logs:
   ```bash
   journalctl -u hypercore-agent-manager -n 50
   ```

### High Latency

**Problem**: Agent responses are slow (> 30s)

**Solutions**:
1. Check agent load:
   ```bash
   curl http://localhost:8080/v1/agents/stats
   ```

2. Verify autoscaler is working:
   ```bash
   journalctl -u agent-autoscaler -n 50
   ```

3. Check Anthropic API status:
   ```bash
   curl https://status.anthropic.com/
   ```

### Autoscaler Not Scaling

**Problem**: Agents not spawning/terminating automatically

**Solutions**:
1. Check Prometheus connectivity:
   ```bash
   curl $PROMETHEUS_URL/api/v1/query?query=agent_active_sessions
   ```

2. Verify metrics are being scraped:
   ```bash
   curl http://localhost:8080/metrics | grep agent_active_sessions
   ```

3. Check autoscaler configuration:
   ```bash
   systemctl status agent-autoscaler
   journalctl -u agent-autoscaler -f
   ```

### Out of Memory (OOM)

**Problem**: Agent containers getting killed

**Solutions**:
1. Increase memory allocation:
   ```bash
   # Edit spawn request
   "memory": 16384  # 16GB instead of 8GB
   ```

2. Monitor memory usage:
   ```bash
   curl http://localhost:9090/metrics | grep container_memory_usage_bytes
   ```

3. Reduce concurrent sessions per agent:
   ```bash
   # Edit .env
   MAX_CONCURRENT_REQUESTS=50  # Instead of 100
   ```

## Load Testing

### Test Suite

A comprehensive load testing suite is available at `tests/load-test-claude-agents.py`.

**Run load test for 10k users**:
```bash
cd tests
python load-test-claude-agents.py \
  --users 10000 \
  --duration 300 \
  --ramp-up 60 \
  --api-url http://localhost:8080
```

**Expected Results**:
- Throughput: 1000+ req/sec
- p95 latency: < 3s
- p99 latency: < 10s
- Error rate: < 1%

## Support & Resources

- **Documentation**: [docs/](../docs/)
- **Examples**: [examples/](../examples/)
- **Issues**: https://github.com/vistara-labs/hypercore/issues
- **Slack**: #hypercore-support

## License

See [LICENSE](../LICENSE) file for details.

---

**Built with Hypercore** - Secure, scalable microVM infrastructure
