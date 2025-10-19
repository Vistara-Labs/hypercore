# Claude Agent SDK - Quick Reference Card

## 🚀 One-Command Deploy
```bash
export ANTHROPIC_API_KEY="sk-ant-..." && ./scripts/deploy-claude-agents.sh
```

## 📡 API Endpoints

### Agent Manager (Port 8080)
```bash
# Health
GET /health
GET /ready
GET /metrics

# Agent Lifecycle
POST   /v1/agents/spawn      # Create agent
DELETE /v1/agents/delete     # Delete agent
GET    /v1/agents/get        # Get agent info
GET    /v1/agents/list       # List user's agents
GET    /v1/agents/stats      # Global statistics
```

### Agent Instance (Per Agent URL)
```bash
# Chat
POST /v1/agent/chat          # Send message
GET  /health                 # Health check
GET  /metrics                # Prometheus metrics

# Session Management
DELETE /v1/agent/session/{id}  # Delete session
GET    /v1/agent/stats         # Agent stats
```

## 💻 Common Commands

### Spawn Agent
```bash
curl -X POST http://localhost:8080/v1/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "anthropic_api_key": "sk-ant-...",
    "cores": 4,
    "memory": 8192,
    "max_concurrent": 100
  }'
```

### Chat with Agent
```bash
curl -X POST https://AGENT_ID.deployments.vistara.dev/v1/agent/chat \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Your question here",
    "user_id": "user-123",
    "max_tokens": 4096,
    "stream": false
  }'
```

### List Agents
```bash
curl http://localhost:8080/v1/agents/list?user_id=user-123
```

### Delete Agent
```bash
curl -X DELETE http://localhost:8080/v1/agents/delete?agent_id=AGENT_ID
```

### Check Statistics
```bash
curl http://localhost:8080/v1/agents/stats
```

## 🔍 Monitoring

### Check Service Status
```bash
systemctl status hypercore-agent-manager
systemctl status agent-autoscaler
```

### View Logs
```bash
journalctl -u hypercore-agent-manager -f
journalctl -u agent-autoscaler -f
```

### Check Metrics
```bash
curl http://localhost:8080/metrics | grep agent_
curl http://localhost:9090/api/v1/query?query=agent_active_sessions
```

### Key Metrics (Prometheus)
```promql
agent_active_sessions                # Current sessions
rate(agent_requests_total[5m])       # Request rate
histogram_quantile(0.95, ...)        # p95 latency
rate(agent_errors_total[5m])         # Error rate
hypercore_agent_active_count         # Agent count
rate(agent_token_usage_total[5m])    # Token usage
```

## 🧪 Testing

### Health Check
```bash
curl http://localhost:8080/health
```

### Load Test (100 users)
```bash
./tests/load-test-claude-agents.py \
  --api-url http://localhost:8080 \
  --anthropic-key "$ANTHROPIC_API_KEY" \
  --users 100 \
  --requests 10
```

### Full Scale Test (10k users)
```bash
./tests/load-test-claude-agents.py \
  --api-url http://localhost:8080 \
  --anthropic-key "$ANTHROPIC_API_KEY" \
  --users 10000 \
  --requests 5 \
  --ramp-up 300 \
  --output results.json
```

## 🔧 Configuration Files

| File | Purpose |
|------|---------|
| `.env` | Runtime configuration |
| `Dockerfile` | Container build |
| `requirements.txt` | Python dependencies |
| `prometheus-agent-rules.yml` | Alert rules |
| `grafana-agent-dashboard.json` | Grafana dashboard |

## 🎯 Environment Variables

### Critical
```bash
ANTHROPIC_API_KEY=sk-ant-...     # Required
HYPERCORE_ADDR=localhost:8000    # Hypercore API
```

### Performance
```bash
MAX_CONCURRENT_REQUESTS=100      # Sessions per agent
REQUEST_TIMEOUT=300              # Timeout in seconds
WORKER_COUNT=4                   # Worker processes
```

### Scaling
```bash
MIN_AGENTS=10                    # Minimum pool size
MAX_AGENTS=1000                  # Maximum for 10k users
SCALE_UP_THRESHOLD=0.80          # Scale up at 80%
SCALE_DOWN_THRESHOLD=0.30        # Scale down at 30%
```

## 🚨 Troubleshooting

### Agent Won't Spawn
```bash
# 1. Check hypercore
curl http://$HYPERCORE_ADDR/health

# 2. Check logs
journalctl -u hypercore-agent-manager -n 50

# 3. Verify image
docker images | grep claude-agent
```

### High Latency
```bash
# 1. Check load
curl http://localhost:8080/v1/agents/stats

# 2. Check autoscaler
journalctl -u agent-autoscaler -n 50

# 3. Monitor metrics
curl http://localhost:9090/api/v1/query?query=agent_active_sessions
```

### Service Down
```bash
# Restart services
sudo systemctl restart hypercore-agent-manager
sudo systemctl restart agent-autoscaler

# Check status
systemctl status hypercore-agent-manager
```

## 📊 Capacity Reference

| Users | Agents | CPU Cores | Memory (GB) |
|-------|--------|-----------|-------------|
| 1,000 | 10 | 40 | 80 |
| 5,000 | 50 | 200 | 400 |
| 10,000 | 100 | 400 | 800 |
| 50,000 | 500 | 2,000 | 4,000 |
| 100,000 | 1,000 | 4,000 | 8,000 |

**Formula**: `agents = users / 100` (100 sessions per agent)

## 🔐 Security Checklist

- [ ] Unique API keys per user
- [ ] TLS/HTTPS enabled
- [ ] Rate limiting configured
- [ ] Resource quotas set
- [ ] Monitoring alerts active
- [ ] Logs being collected
- [ ] Backups configured

## 📞 Quick Links

- **Docs**: [docs/CLAUDE_AGENT_SDK_INTEGRATION.md](../../docs/CLAUDE_AGENT_SDK_INTEGRATION.md)
- **Deploy**: [scripts/deploy-claude-agents.sh](../../scripts/deploy-claude-agents.sh)
- **Monitor**: [monitoring/grafana-agent-dashboard.json](../../monitoring/grafana-agent-dashboard.json)
- **Test**: [tests/load-test-claude-agents.py](../../tests/load-test-claude-agents.py)

## 💡 Pro Tips

1. **Always use separate API keys per user** for security and usage tracking
2. **Monitor token costs** - they often exceed infrastructure costs
3. **Scale up aggressively, scale down conservatively** to avoid capacity issues
4. **Keep warm pool of 10 agents** for instant availability
5. **Set up alerts before going live** - don't wait for problems
6. **Test with 2x expected load** to ensure headroom
7. **Use Redis for multi-instance deployments** for session persistence

## 🎓 Next Steps

1. ✅ Deploy to staging
2. ✅ Run load tests
3. ✅ Configure monitoring
4. ✅ Set up alerts
5. ✅ Review logs
6. ✅ Test failover
7. ✅ Deploy to production
8. ✅ Monitor closely
9. ✅ Optimize based on usage
10. ✅ Scale as needed

---

**Need help?** See [CLAUDE_AGENT_INTEGRATION_SUMMARY.md](../../CLAUDE_AGENT_INTEGRATION_SUMMARY.md)
