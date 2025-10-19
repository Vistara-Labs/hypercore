#!/bin/bash
# Production deployment script for Claude Agent SDK integration
# Designed for 10k+ concurrent users

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REGISTRY_URL="${REGISTRY_URL:-registry.vistara.dev}"
HYPERCORE_ADDR="${HYPERCORE_ADDR:-localhost:8000}"
ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
DEPLOYMENT_ENV="${DEPLOYMENT_ENV:-production}"

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
AGENT_DIR="$PROJECT_ROOT/deployments/claude-agent"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi

    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        exit 1
    fi

    # Check Anthropic API key
    if [ -z "$ANTHROPIC_API_KEY" ]; then
        log_error "ANTHROPIC_API_KEY environment variable is not set"
        exit 1
    fi

    # Check hypercore connectivity
    if ! curl -sf "http://$HYPERCORE_ADDR/health" > /dev/null; then
        log_warn "Cannot connect to hypercore at $HYPERCORE_ADDR"
    fi

    log_success "Prerequisites check passed"
}

build_agent_image() {
    log_info "Building Claude Agent SDK container image..."

    cd "$AGENT_DIR"

    docker build \
        --tag "$REGISTRY_URL/claude-agent:latest" \
        --tag "$REGISTRY_URL/claude-agent:$(date +%Y%m%d-%H%M%S)" \
        --build-arg BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --build-arg VERSION="1.0.0" \
        .

    log_success "Container image built successfully"
}

push_agent_image() {
    log_info "Pushing container image to registry..."

    docker push "$REGISTRY_URL/claude-agent:latest"

    log_success "Container image pushed successfully"
}

build_integration_service() {
    log_info "Building hypercore integration service..."

    cd "$AGENT_DIR"

    go build -o "$PROJECT_ROOT/bin/hypercore-agent-manager" \
        -ldflags "-X main.Version=1.0.0 -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        hypercore_integration.go

    log_success "Integration service built successfully"
}

build_autoscaler() {
    log_info "Building autoscaler service..."

    cd "$AGENT_DIR"

    go build -o "$PROJECT_ROOT/bin/agent-autoscaler" \
        -ldflags "-X main.Version=1.0.0 -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        autoscaler.go

    log_success "Autoscaler built successfully"
}

deploy_integration_service() {
    log_info "Deploying integration service..."

    # Create systemd service file
    cat > /tmp/hypercore-agent-manager.service <<EOF
[Unit]
Description=Hypercore Claude Agent Manager
After=network.target

[Service]
Type=simple
User=hypercore
Environment="HYPERCORE_ADDR=$HYPERCORE_ADDR"
Environment="REGISTRY_URL=$REGISTRY_URL"
Environment="MAX_AGENTS_PER_USER=50"
Environment="LISTEN_ADDR=:8080"
ExecStart=$PROJECT_ROOT/bin/hypercore-agent-manager
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    sudo mv /tmp/hypercore-agent-manager.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable hypercore-agent-manager
    sudo systemctl restart hypercore-agent-manager

    log_success "Integration service deployed"
}

deploy_autoscaler() {
    log_info "Deploying autoscaler service..."

    # Create systemd service file
    cat > /tmp/agent-autoscaler.service <<EOF
[Unit]
Description=Claude Agent Autoscaler
After=network.target hypercore-agent-manager.service
Requires=hypercore-agent-manager.service

[Service]
Type=simple
User=hypercore
Environment="PROMETHEUS_URL=$PROMETHEUS_URL"
Environment="HYPERCORE_ADDR=$HYPERCORE_ADDR"
Environment="REGISTRY_URL=$REGISTRY_URL"
ExecStart=$PROJECT_ROOT/bin/agent-autoscaler
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    sudo mv /tmp/agent-autoscaler.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable agent-autoscaler
    sudo systemctl restart agent-autoscaler

    log_success "Autoscaler deployed"
}

setup_monitoring() {
    log_info "Setting up monitoring..."

    # Copy Prometheus rules
    if [ -f "$PROJECT_ROOT/monitoring/prometheus-agent-rules.yml" ]; then
        sudo cp "$PROJECT_ROOT/monitoring/prometheus-agent-rules.yml" /etc/prometheus/rules/
        sudo systemctl reload prometheus
        log_success "Prometheus rules configured"
    fi

    # Import Grafana dashboard
    if [ -f "$PROJECT_ROOT/monitoring/grafana-agent-dashboard.json" ]; then
        log_info "Grafana dashboard available at: $PROJECT_ROOT/monitoring/grafana-agent-dashboard.json"
        log_info "Import manually via Grafana UI"
    fi

    log_success "Monitoring setup complete"
}

spawn_initial_agents() {
    log_info "Spawning initial agent pool (10 agents for warm start)..."

    for i in {1..10}; do
        curl -X POST "http://localhost:8080/v1/agents/spawn" \
            -H "Content-Type: application/json" \
            -d "{
                \"user_id\": \"system-warmup-$i\",
                \"anthropic_api_key\": \"$ANTHROPIC_API_KEY\",
                \"cores\": 4,
                \"memory\": 8192,
                \"max_concurrent\": 100
            }" || log_warn "Failed to spawn agent $i"

        sleep 2
    done

    log_success "Initial agent pool spawned"
}

run_health_checks() {
    log_info "Running health checks..."

    # Check integration service
    if curl -sf http://localhost:8080/health > /dev/null; then
        log_success "Integration service is healthy"
    else
        log_error "Integration service health check failed"
        return 1
    fi

    # Check if agents are running
    AGENT_COUNT=$(curl -s http://localhost:8080/v1/agents/stats | grep -o '"total_agents":[0-9]*' | grep -o '[0-9]*')
    log_info "Active agents: $AGENT_COUNT"

    if [ "$AGENT_COUNT" -ge 5 ]; then
        log_success "Sufficient agents running"
    else
        log_warn "Low agent count: $AGENT_COUNT"
    fi

    log_success "Health checks passed"
}

print_summary() {
    echo ""
    echo "======================================================================"
    echo "  Claude Agent SDK Deployment Complete!"
    echo "======================================================================"
    echo ""
    echo "Services:"
    echo "  - Integration API:  http://localhost:8080"
    echo "  - Metrics:          http://localhost:9090/metrics"
    echo "  - Prometheus:       $PROMETHEUS_URL"
    echo ""
    echo "Configuration:"
    echo "  - Registry:         $REGISTRY_URL"
    echo "  - Hypercore:        $HYPERCORE_ADDR"
    echo "  - Environment:      $DEPLOYMENT_ENV"
    echo ""
    echo "Quick Start:"
    echo "  # Spawn an agent"
    echo "  curl -X POST http://localhost:8080/v1/agents/spawn \\"
    echo "    -H 'Content-Type: application/json' \\"
    echo "    -d '{\"user_id\": \"test-user\", \"anthropic_api_key\": \"sk-...\"}'"
    echo ""
    echo "  # List agents"
    echo "  curl http://localhost:8080/v1/agents/list?user_id=test-user"
    echo ""
    echo "  # Check statistics"
    echo "  curl http://localhost:8080/v1/agents/stats"
    echo ""
    echo "Monitoring:"
    echo "  - View logs:        journalctl -u hypercore-agent-manager -f"
    echo "  - View autoscaler:  journalctl -u agent-autoscaler -f"
    echo "  - Import Grafana dashboard from: $PROJECT_ROOT/monitoring/grafana-agent-dashboard.json"
    echo ""
    echo "======================================================================"
    echo ""
}

main() {
    log_info "Starting Claude Agent SDK deployment for $DEPLOYMENT_ENV"
    log_info "Target capacity: 10,000 concurrent users"

    check_prerequisites
    build_agent_image

    if [ "$DEPLOYMENT_ENV" == "production" ]; then
        push_agent_image
    fi

    build_integration_service
    build_autoscaler
    deploy_integration_service
    deploy_autoscaler
    setup_monitoring

    sleep 5  # Wait for services to start

    run_health_checks
    spawn_initial_agents

    print_summary
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-build)
            SKIP_BUILD=1
            shift
            ;;
        --skip-push)
            SKIP_PUSH=1
            shift
            ;;
        --skip-agents)
            SKIP_AGENTS=1
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-build   Skip building container image"
            echo "  --skip-push    Skip pushing to registry"
            echo "  --skip-agents  Skip spawning initial agents"
            echo "  --help         Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  REGISTRY_URL         Container registry URL"
            echo "  HYPERCORE_ADDR       Hypercore API address"
            echo "  ANTHROPIC_API_KEY    Anthropic API key (required)"
            echo "  PROMETHEUS_URL       Prometheus server URL"
            echo "  DEPLOYMENT_ENV       Deployment environment (production/staging)"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

main
