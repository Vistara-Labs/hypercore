# Claude Agent SDK Integration - Architecture

## System Overview

```
                                    ┌─────────────────────────┐
                                    │   10,000+ Users         │
                                    └──────────┬──────────────┘
                                               │
                                               │ HTTPS/TLS
                                               │
                                               ▼
┌────────────────────────────────────────────────────────────────────────┐
│                         Load Balancer / API Gateway                    │
│                    (Rate Limiting, Auth, TLS Termination)              │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
                                   │ HTTP
                                   │
                   ┌───────────────┴──────────────┐
                   │                              │
                   ▼                              ▼
┌──────────────────────────────────┐  ┌──────────────────────────────────┐
│  Hypercore Agent Manager (Go)    │  │  Hypercore Agent Manager (Go)    │
│  Port: 8080                      │  │  Port: 8080                      │
│  ┌────────────────────────────┐  │  │  ┌────────────────────────────┐  │
│  │ • Spawn/Delete Agents      │  │  │  │ • Spawn/Delete Agents      │  │
│  │ • Route Requests           │  │  │  │ • Route Requests           │  │
│  │ • Health Monitoring        │  │  │  │ • Health Monitoring        │  │
│  │ • User Quotas              │  │  │  │ • User Quotas              │  │
│  │ • Prometheus Metrics       │  │  │  │ • Prometheus Metrics       │  │
│  └────────────────────────────┘  │  │  └────────────────────────────┘  │
└──────────────┬───────────────────┘  └──────────────┬───────────────────┘
               │                                     │
               │ gRPC/HTTP to Hypercore              │
               │                                     │
               ▼                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│                         Hypercore Cluster                              │
│                    (MicroVM Orchestration Layer)                       │
└──────────────────────────┬─────────────────────────────────────────────┘
                           │
                           │ Spawns MicroVMs
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Claude Agent  │  │ Claude Agent  │  │ Claude Agent  │  ... (10-1000 agents)
│  (MicroVM 1)  │  │  (MicroVM 2)  │  │  (MicroVM N)  │
│               │  │               │  │               │
│ ┌───────────┐ │  │ ┌───────────┐ │  │ ┌───────────┐ │
│ │ FastAPI   │ │  │ │ FastAPI   │ │  │ │ FastAPI   │ │
│ │ Server    │ │  │ │ Server    │ │  │ │ Server    │ │
│ │ :8080     │ │  │ │ :8080     │ │  │ │ :8080     │ │
│ └───────────┘ │  │ └───────────┘ │  │ └───────────┘ │
│               │  │               │  │               │
│ ┌───────────┐ │  │ ┌───────────┐ │  │ ┌───────────┐ │
│ │ Agent     │ │  │ │ Agent     │ │  │ │ Agent     │ │
│ │ Manager   │ │  │ │ Manager   │ │  │ │ Manager   │ │
│ │ (Python)  │ │  │ │ (Python)  │ │  │ │ (Python)  │ │
│ └───────────┘ │  │ └───────────┘ │  │ └───────────┘ │
│               │  │               │  │               │
│ Resources:    │  │ Resources:    │  │ Resources:    │
│ • 4 CPU       │  │ • 4 CPU       │  │ • 4 CPU       │
│ • 8GB RAM     │  │ • 8GB RAM     │  │ • 8GB RAM     │
│ • 100 sess    │  │ • 100 sess    │  │ • 100 sess    │
│ • TAP net     │  │ • TAP net     │  │ • TAP net     │
└───────┬───────┘  └───────┬───────┘  └───────┬───────┘
        │                  │                  │
        └──────────────────┼──────────────────┘
                           │
                           │ Claude API Calls
                           │
                           ▼
┌────────────────────────────────────────────────────────────────────────┐
│                       Anthropic Claude API                             │
│                  (claude-3-5-sonnet-20241022)                         │
└────────────────────────────────────────────────────────────────────────┘


┌────────────────────────────────────────────────────────────────────────┐
│                         Monitoring & Observability                     │
│                                                                        │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐           │
│  │ Prometheus   │───▶│  Grafana     │───▶│  Alerting    │           │
│  │ :9090        │    │  :3000       │    │  (PagerDuty) │           │
│  │              │    │              │    │              │           │
│  │ • Metrics    │    │ • Dashboards │    │ • Alerts     │           │
│  │ • Rules      │    │ • Viz        │    │ • Incidents  │           │
│  │ • Alerts     │    │ • Reports    │    │ • Oncall     │           │
│  └──────────────┘    └──────────────┘    └──────────────┘           │
│         ▲                                                              │
│         │ Scrape Metrics (every 15s)                                  │
│         │                                                              │
│  ┌──────┴───────────────────────────────────────────────────┐        │
│  │  Agent Manager :8080/metrics                             │        │
│  │  Agent Instances :8080/metrics                           │        │
│  │  Autoscaler :9090/metrics                                │        │
│  └──────────────────────────────────────────────────────────┘        │
└────────────────────────────────────────────────────────────────────────┘


┌────────────────────────────────────────────────────────────────────────┐
│                         Auto-Scaling System                            │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  Agent Autoscaler (Go)                                       │    │
│  │  Port: 9090                                                  │    │
│  │  ┌────────────────────────────────────────────────────────┐ │    │
│  │  │ 1. Query Prometheus for metrics                        │ │    │
│  │  │    • agent_active_sessions                             │ │    │
│  │  │    • hypercore_agent_active_count                      │ │    │
│  │  │    • rate(agent_requests_total[5m])                    │ │    │
│  │  │    • container CPU/memory usage                        │ │    │
│  │  │                                                         │ │    │
│  │  │ 2. Calculate capacity utilization                      │ │    │
│  │  │    utilization = sessions / (agents * 100)             │ │    │
│  │  │                                                         │ │    │
│  │  │ 3. Make scaling decision                               │ │    │
│  │  │    • If utilization > 80%: SCALE UP                    │ │    │
│  │  │    • If utilization < 30%: SCALE DOWN                  │ │    │
│  │  │    • Else: NO ACTION                                   │ │    │
│  │  │                                                         │ │    │
│  │  │ 4. Execute scaling via Agent Manager API               │ │    │
│  │  │    POST /v1/agents/spawn (scale up)                    │ │    │
│  │  │    DELETE /v1/agents/delete (scale down)               │ │    │
│  │  │                                                         │ │    │
│  │  │ 5. Wait for cooldown period                            │ │    │
│  │  │    • Scale up cooldown: 2 minutes                      │ │    │
│  │  │    • Scale down cooldown: 5 minutes                    │ │    │
│  │  └────────────────────────────────────────────────────────┘ │    │
│  │                                                              │    │
│  │  Configuration:                                              │    │
│  │  • Min Agents: 10                                            │    │
│  │  • Max Agents: 1000                                          │    │
│  │  • Target Sessions/Agent: 100                                │    │
│  │  • Evaluation Interval: 30s                                  │    │
│  │  • Aggressive Scaling: true                                  │    │
│  │  • Predictive Scaling: true                                  │    │
│  └──────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### 1. Agent Spawn Flow
```
User Request
    │
    ▼
API Gateway
    │
    ▼
Agent Manager
    │
    ├──▶ Validate Request
    │    (user_id, quotas, resources)
    │
    ├──▶ Call Hypercore Spawn API
    │    POST /spawn
    │    {
    │      imageRef: "registry.vistara.dev/claude-agent:latest",
    │      cores: 4,
    │      memory: 8192,
    │      ports: ["443:8080", "9090:9090"],
    │      env: {ANTHROPIC_API_KEY: "..."}
    │    }
    │
    ▼
Hypercore
    │
    ├──▶ Pull Container Image
    ├──▶ Create MicroVM
    ├──▶ Configure Network (TAP device)
    ├──▶ Mount Storage
    ├──▶ Start Container
    │
    ▼
Agent Container Starts
    │
    ├──▶ Initialize FastAPI Server
    ├──▶ Create AgentManager
    ├──▶ Setup Prometheus Metrics
    ├──▶ Start Health Checks
    │
    ▼
Agent Ready
    │
    ▼
Return to User
    {
      agent_id: "abc123",
      url: "abc123.deployments.vistara.dev",
      status: "running"
    }
```

### 2. Chat Request Flow
```
User Request
    │
    ▼
Agent URL (abc123.deployments.vistara.dev)
    │
    ▼
FastAPI Server (in MicroVM)
    │
    ├──▶ Rate Limiting Check (100/min)
    ├──▶ Create/Get Session
    │
    ▼
Agent Manager
    │
    ├──▶ Add message to history
    ├──▶ Prepare context
    │
    ▼
Anthropic API
    │
    ├──▶ Call Claude API
    ├──▶ Stream/Complete response
    │
    ▼
Process Response
    │
    ├──▶ Extract content
    ├──▶ Update token usage
    ├──▶ Record metrics
    │
    ▼
Return to User
    {
      session_id: "sess-xyz",
      content: "...",
      usage: {tokens: 500},
      status: "completed"
    }
```

### 3. Auto-Scaling Flow
```
Every 30 seconds:

Autoscaler
    │
    ├──▶ Query Prometheus
    │    • Current sessions: 7500
    │    • Current agents: 50
    │    • Request rate: 250/sec
    │
    ▼
Calculate Metrics
    │
    ├──▶ Utilization = 7500 / (50 × 100) = 1.5 (150%)
    ├──▶ Target agents = 7500 / 100 = 75
    ├──▶ Delta = 75 - 50 = 25 agents needed
    │
    ▼
Check Thresholds
    │
    ├──▶ 150% > 80% threshold → SCALE UP
    ├──▶ Check cooldown (OK)
    ├──▶ Check max limit (75 < 1000, OK)
    │
    ▼
Execute Scaling
    │
    ├──▶ Spawn 25 new agents (via Agent Manager API)
    ├──▶ Wait 2 minutes (scale-up cooldown)
    │
    ▼
Monitor Results
    │
    ├──▶ New capacity: 75 agents × 100 = 7500 sessions
    ├──▶ New utilization: 7500 / 7500 = 100%
    ├──▶ Status: Within acceptable range
    │
    ▼
Continue Monitoring...
```

## Component Details

### Agent Manager (Go Service)
- **Purpose**: Orchestrate agent lifecycle
- **Port**: 8080
- **APIs**: Spawn, delete, list, stats
- **Responsibilities**:
  - User quota enforcement
  - Agent health monitoring
  - Metrics collection
  - Request routing

### Agent Container (Python/FastAPI)
- **Purpose**: Run Claude agent instances
- **Port**: 8080 (agent), 9090 (metrics)
- **Components**:
  - FastAPI web server
  - AgentManager (session handling)
  - Health check system
  - Prometheus metrics
- **Capacity**: 100 concurrent sessions

### Autoscaler (Go Service)
- **Purpose**: Automatic capacity management
- **Evaluation**: Every 30 seconds
- **Decisions**: Scale up/down based on load
- **Thresholds**: 80% up, 30% down
- **Limits**: 10-1000 agents

### Monitoring Stack
- **Prometheus**: Metrics collection & querying
- **Grafana**: Visualization & dashboards
- **Alertmanager**: Alert routing & notifications
- **Targets**: All agent instances + manager + autoscaler

## Security Layers

### Layer 1: Network
- Load balancer with TLS termination
- Rate limiting (100 req/min per IP)
- DDoS protection
- Geographic filtering (optional)

### Layer 2: API Gateway
- Authentication (JWT/OAuth)
- Authorization (RBAC)
- Request validation
- Audit logging

### Layer 3: Agent Manager
- User quota enforcement (50 agents/user)
- Resource limit validation
- API key encryption
- Session isolation

### Layer 4: MicroVM
- Hardware-enforced isolation
- Separate kernel per VM
- TAP network isolation
- cgroup resource limits

### Layer 5: Container
- Non-root user (UID 1000)
- Read-only root filesystem
- No privilege escalation
- Dropped capabilities

## Scalability Considerations

### Horizontal Scaling
- **Agent Manager**: Run 2-3 instances with load balancer
- **Agents**: Auto-scale from 10 to 1000
- **Monitoring**: Prometheus federation for multi-cluster

### Vertical Scaling
- **Agent Resources**: 4-8 cores, 8-16GB RAM per agent
- **Sessions per Agent**: 50-200 based on workload
- **Database**: Redis for session state (multi-instance)

### Geographic Distribution
- **Multi-region**: Deploy in US, EU, Asia
- **CDN**: Static assets via CloudFlare
- **Latency Optimization**: Route to nearest region

## Cost Optimization

### Infrastructure
- Use spot instances where possible (30-70% savings)
- Right-size agent resources based on actual usage
- Scale down aggressively during off-hours
- Use reserved instances for base capacity

### API Costs
- Implement response caching (Redis)
- Use cheaper models for simple tasks
- Set token limits per user/request
- Monitor and alert on unusual usage

### Monitoring
- Use Prometheus recording rules to reduce query load
- Retain metrics for 30 days, then downsample
- Export expensive logs to cheaper storage (S3)

## Disaster Recovery

### Backup Strategy
- **Configuration**: Git repository
- **Metrics**: Prometheus snapshots (daily)
- **Logs**: Ship to S3 or log aggregation service
- **State**: Redis backups (if using for sessions)

### Recovery Procedures
1. **Agent Manager Down**: Load balancer redirects to healthy instance
2. **Agent Down**: Auto-heal via hypercore, spawn replacement
3. **Autoscaler Down**: Manual scaling via API
4. **Complete Outage**: Deploy from scratch using deploy script (15 min)

### RTO/RPO Targets
- **Recovery Time Objective (RTO)**: 15 minutes
- **Recovery Point Objective (RPO)**: 1 hour
- **High Availability**: 99.9% uptime

---

**Ready to deploy?** See [README.md](README.md) for quick start guide.
