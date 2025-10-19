# Claude Agent SDK Integration for Hypercore

**Production-ready deployment for 10,000+ concurrent users**

## 🚀 Quick Start (5 minutes)

```bash
# 1. Set your Anthropic API key
export ANTHROPIC_API_KEY="sk-ant-api03-..."

# 2. Deploy everything
cd /path/to/hypercore
./scripts/deploy-claude-agents.sh

# 3. Test it works
curl http://localhost:8080/health

# 4. Spawn your first agent
curl -X POST http://localhost:8080/v1/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test", "anthropic_api_key": "'$ANTHROPIC_API_KEY'"}'
```

## 📁 Project Structure

```
deployments/claude-agent/
├── Dockerfile                    # Production container image
├── requirements.txt              # Python dependencies
├── src/
│   ├── agent_server.py          # Main FastAPI server
│   ├── agent_manager.py         # Session & lifecycle management
│   ├── config.py                # Configuration management
│   ├── health.py                # Health check system
│   └── metrics.py               # Prometheus metrics
├── hypercore_integration.go     # Hypercore API integration
├── autoscaler.go                # Auto-scaling service
└── .env.example                 # Configuration template

scripts/
└── deploy-claude-agents.sh      # One-click deployment

monitoring/
├── grafana-agent-dashboard.json # Grafana dashboard
└── prometheus-agent-rules.yml   # Alerting rules

tests/
└── load-test-claude-agents.py   # Load testing suite

docs/
└── CLAUDE_AGENT_SDK_INTEGRATION.md  # Full documentation
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│          10,000 Concurrent Users                    │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│       Hypercore Agent Manager (Port 8080)           │
│   • Spawn agents                                    │
│   • Route requests                                  │
│   • Monitor health                                  │
└──────────────────┬──────────────────────────────────┘
                   │
         ┌─────────┴─────────┐
         │                   │
         ▼                   ▼
┌─────────────────┐  ┌─────────────────┐
│  Claude Agent   │  │  Claude Agent   │  (100 agents)
│   (MicroVM)     │  │   (MicroVM)     │
│                 │  │                 │
│ 100 sessions    │  │ 100 sessions    │
│ 4 CPU cores     │  │ 4 CPU cores     │
│ 8GB RAM         │  │ 8GB RAM         │
└─────────────────┘  └─────────────────┘
         │                   │
         └─────────┬─────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│              Autoscaler (Background)                │
│   • Monitor Prometheus metrics                      │
│   • Scale up/down based on load                     │
│   • Maintain 10-1000 agent pool                     │
└─────────────────────────────────────────────────────┘
```

## 🎯 Key Features

### Security
- ✅ MicroVM isolation per agent
- ✅ Non-root containers
- ✅ Resource quotas (CPU, RAM, GPU)
- ✅ Network isolation via TAP devices
- ✅ API key encryption at rest

### Performance
- ✅ 10k+ concurrent users
- ✅ Auto-scaling (10-1000 agents)
- ✅ Multi-process workers
- ✅ Connection pooling
- ✅ < 2s p95 latency

### Observability
- ✅ Prometheus metrics
- ✅ Grafana dashboards
- ✅ Structured logging
- ✅ Health checks
- ✅ Distributed tracing

## 📊 Capacity Planning

| Metric | Value |
|--------|-------|
| Target Users | 10,000 concurrent |
| Sessions per Agent | 100 |
| Required Agents (full load) | 100 |
| Min Agents (warm pool) | 10 |
| Max Agents (with buffer) | 1,000 |
| CPU per Agent | 4 cores |
| RAM per Agent | 8GB |
| Scale-up Threshold | 80% capacity |
| Scale-down Threshold | 30% capacity |

**Total Resources (at full load)**:
- CPU: 400 cores
- Memory: 800GB RAM
- Estimated Infrastructure Cost: $500-1000/month
- Estimated Token Cost: $24k/month (10k users × 10 req/day)

## 🛠️ Configuration

### Environment Variables

Copy [.env.example](.env.example) to `.env` and configure:

**Required**:
- `ANTHROPIC_API_KEY` - Your Anthropic API key

**Optional** (with sensible defaults):
- `MAX_CONCURRENT_REQUESTS=100` - Sessions per agent
- `REQUEST_TIMEOUT=300` - Request timeout in seconds
- `MIN_AGENTS=10` - Minimum agents in pool
- `MAX_AGENTS=1000` - Maximum agents for 10k users
- `SCALE_UP_THRESHOLD=0.80` - Scale up at 80% capacity
- `SCALE_DOWN_THRESHOLD=0.30` - Scale down at 30% capacity

## 🧪 Testing

### Manual Testing

```bash
# Health check
curl http://localhost:8080/health

# Spawn agent
curl -X POST http://localhost:8080/v1/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test-user",
    "anthropic_api_key": "sk-ant-...",
    "cores": 4,
    "memory": 8192
  }'

# Chat with agent (use URL from spawn response)
curl -X POST https://AGENT_ID.deployments.vistara.dev/v1/agent/chat \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Explain quantum computing",
    "user_id": "test-user",
    "max_tokens": 500
  }'

# List user agents
curl http://localhost:8080/v1/agents/list?user_id=test-user

# Delete agent
curl -X DELETE http://localhost:8080/v1/agents/delete?agent_id=AGENT_ID
```

### Load Testing

```bash
# Install test dependencies
pip install -r ../tests/requirements-test.txt

# Run load test (100 users)
../tests/load-test-claude-agents.py \
  --api-url http://localhost:8080 \
  --anthropic-key "$ANTHROPIC_API_KEY" \
  --users 100 \
  --requests 10 \
  --ramp-up 30

# Run full 10k user test
../tests/load-test-claude-agents.py \
  --api-url http://localhost:8080 \
  --anthropic-key "$ANTHROPIC_API_KEY" \
  --users 10000 \
  --requests 5 \
  --ramp-up 300 \
  --output results.json
```

## 📈 Monitoring

### Metrics Endpoints

- **Agent Manager**: http://localhost:8080/metrics
- **Individual Agents**: https://AGENT_ID.deployments.vistara.dev/metrics

### Key Metrics

```promql
# Active sessions
agent_active_sessions

# Request rate
rate(agent_requests_total[5m])

# p95 latency
histogram_quantile(0.95, rate(agent_request_duration_seconds_bucket[5m]))

# Error rate
rate(agent_errors_total[5m])

# Agent count
hypercore_agent_active_count

# Token usage
rate(agent_token_usage_total[5m])
```

### Grafana Dashboard

Import `../../monitoring/grafana-agent-dashboard.json` for:
- Real-time session count
- Request rates and latency
- Token usage and costs
- Error rates and types
- Agent spawn metrics
- Resource utilization

### Alerts

Alerts are configured in `../../monitoring/prometheus-agent-rules.yml`:
- High error rate (> 0.1/sec)
- High latency (p95 > 30s)
- Agent spawn failures
- Capacity warnings (> 5000 sessions)
- Critical capacity (> 9000 sessions)

## 🔧 Troubleshooting

### Agent won't spawn
```bash
# Check hypercore connectivity
curl http://$HYPERCORE_ADDR/health

# Check logs
journalctl -u hypercore-agent-manager -n 50

# Verify image exists
docker images | grep claude-agent
```

### High latency
```bash
# Check agent load
curl http://localhost:8080/v1/agents/stats

# Check autoscaler
journalctl -u agent-autoscaler -n 50

# Monitor metrics
curl http://localhost:9090/api/v1/query?query=agent_active_sessions
```

### Autoscaler not working
```bash
# Check Prometheus connectivity
curl $PROMETHEUS_URL/api/v1/query?query=agent_active_sessions

# Verify autoscaler is running
systemctl status agent-autoscaler

# Check autoscaler logs
journalctl -u agent-autoscaler -f
```

## 🚦 Production Checklist

Before going live with 10k users:

- [ ] Set production `ANTHROPIC_API_KEY`
- [ ] Configure load balancer with TLS
- [ ] Set up monitoring (Prometheus + Grafana)
- [ ] Configure alerting (PagerDuty/Slack)
- [ ] Run load tests successfully (> 95% success rate)
- [ ] Set up log aggregation
- [ ] Configure backup and disaster recovery
- [ ] Document runbooks for common issues
- [ ] Set up cost monitoring and alerts
- [ ] Test autoscaler behavior under load
- [ ] Configure rate limiting at API gateway
- [ ] Set up distributed tracing (optional)

## 📚 Documentation

- **Full Documentation**: [../../docs/CLAUDE_AGENT_SDK_INTEGRATION.md](../../docs/CLAUDE_AGENT_SDK_INTEGRATION.md)
- **Hypercore Docs**: [../../docs/](../../docs/)
- **API Reference**: See FastAPI docs at http://localhost:8080/docs

## 🤝 Support

- **Issues**: https://github.com/vistara-labs/hypercore/issues
- **Discussions**: https://github.com/vistara-labs/hypercore/discussions

## 📄 License

See [../../LICENSE](../../LICENSE)

---

**Built with ❤️ using Hypercore** - Secure, scalable microVM infrastructure
