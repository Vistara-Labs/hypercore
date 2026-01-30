# Running Agentic Workloads on Hypercore

Hypercore microVMs are ideal for running AI agents that need full isolation, persistent connections, and unrestricted execution environments.

## Why microVMs for AI Agents?

| Requirement | V8 Isolates | Containers | microVMs |
|-------------|-------------|------------|----------|
| Full Linux environment | No | Yes | Yes |
| Isolation boundary | Process | Kernel (cgroups/namespaces) | Hardware (KVM) |
| Long-lived connections | Limited | Yes | Yes |
| Arbitrary code execution | No | Yes | Yes |
| Resource guarantees | Shared | cgroups | Explicitly allocated |

**AI agents need:**
- Persistent WebSocket connections for streaming responses
- Ability to run tools (shell commands, file operations)
- Full filesystem for agent workspace
- Isolated execution for running untrusted code
- Resource guarantees (dedicated CPU/memory)

## Architecture: One Agent Per User

```
┌─────────────────────────────────────────────────────────────────┐
│                       Hypercore Cluster                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   Node 1     │  │   Node 2     │  │   Node 3     │           │
│  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │           │
│  │  │ Agent  │  │  │  │ Agent  │  │  │  │ Agent  │  │           │
│  │  │ User A │  │  │  │ User C │  │  │  │ User E │  │           │
│  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │           │
│  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │           │
│  │  │ Agent  │  │  │  │ Agent  │  │  │  │ Agent  │  │           │
│  │  │ User B │  │  │  │ User D │  │  │  │ User F │  │           │
│  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                *.<your-domain>
                     (TLS termination)
```

Each user gets:
- **Dedicated microVM**: Configurable CPU cores and RAM
- **Isolated network**: Separate IP, bridged networking
- **Private filesystem**: Agent workspace, configs, session data
- **Own credentials**: User's API keys, not shared

## Example: Spawning an AI Gateway

The following is an illustrative example of the spawn API:

```bash
curl -X POST https://gateway:8443/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "image_ref": "your-registry/agent-image:latest",
    "cores": 2,
    "memory": 2048,
    "ports": {"443": 8080},
    "env": ["API_KEY=..."]
  }'
```

**Response:**
```json
{
  "response": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "url": "a1b2c3d4-e5f6-7890-abcd-ef1234567890.<your-domain>"
  }
}
```

The agent is now accessible at `https://<id>.<your-domain>`.

## Resource Configuration

### Recommended specs:

| Workload Type | CPU | Memory | Notes |
|---------------|-----|--------|-------|
| Chat-only agent | 1 | 1GB | Minimal, handles conversations |
| Coding agent | 2 | 2GB | Space for repos, builds |
| Multi-tool agent | 4 | 4GB | Browser automation, API calls |
| Heavy compute | 8 | 8GB | Local inference, large codebases |

## Persistent Storage

### Current State

By default, the microVM filesystem is ephemeral - data is lost on restart. This works for:
- Stateless chat agents
- Short-lived tasks
- Demo instances

### Workarounds

Persist state externally:
- **Database**: External PostgreSQL, SQLite over network
- **Object storage**: S3, GCS, R2
- **Cache**: Redis, Memcached
- **Git**: Clone on startup, push on shutdown

### Roadmap

Persistent volume support is planned - containerd already supports block device snapshots, the spawn API needs to expose volume configuration.

## Security Model

### Isolation Guarantees

| Resource | Isolation |
|----------|-----------|
| CPU | vCPU allocation enforced by hypervisor |
| Memory | Hardware-enforced limits |
| Filesystem | Separate rootfs per VM |
| Network | Separate IP, bridged interface |
| Processes | Full VM boundary |

### Current Limitations

Documented in [cluster.md](cluster.md):
- Guest can access host network ports (network isolation planned)
- Secrets passed via environment variables (vault integration planned)
- Workloads don't reschedule on node failure (HA planned)

## Building Agent Images

### Dockerfile Template

```dockerfile
FROM node:22-bookworm-slim

COPY . /app
WORKDIR /app
RUN npm install --production

EXPOSE 8080
CMD ["node", "server.js"]
```

### Best Practices

1. **Small base images**: Use `-slim` or `-alpine` variants
2. **Pre-install dependencies**: Don't install at runtime
3. **Health endpoint**: Expose `/health` for readiness checks
4. **Graceful shutdown**: Handle SIGTERM
5. **Non-root user**: Run as non-root when possible

## API Reference

### Spawn

```http
POST /spawn
Content-Type: application/json

{
  "image_ref": "registry/image:tag",
  "cores": 2,
  "memory": 2048,
  "ports": {"443": 8080},
  "env": ["KEY=value"]
}
```

### Stop

```http
GET /stop?id=<workload-id>
```

### List

```http
GET /list
```

### Logs

```http
GET /logs?id=<workload-id>
```

## FAQ

**Q: How fast do agents boot?**
A: Firecracker VM boot is ~125ms. Total readiness depends on image size and application startup time.

**Q: What happens if a node dies?**
A: Currently, workloads are lost. Automatic rescheduling is on the roadmap.

**Q: Can agents communicate with each other?**
A: Yes, via their public URLs. Internal networking is planned.

**Q: How do I update a running agent?**
A: Stop and respawn with the new image.

## Links

- [Hypercore](https://github.com/Vistara-Labs/hypercore)
- [Firecracker](https://firecracker-microvm.github.io/)
- [Cloud Hypervisor](https://www.cloudhypervisor.org/)
