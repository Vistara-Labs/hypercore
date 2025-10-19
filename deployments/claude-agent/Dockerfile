# Multi-stage build for Claude Agent SDK
# Production-ready container for 10k+ concurrent users

# Stage 1: Builder
FROM python:3.11-slim as builder

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    make \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create virtual environment
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

# Stage 2: Runtime
FROM python:3.11-slim

# Install runtime dependencies only
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Create non-root user for security
RUN useradd -m -u 1000 -s /bin/bash agent && \
    mkdir -p /app /app/logs /app/data && \
    chown -R agent:agent /app

WORKDIR /app

# Copy application code
COPY --chown=agent:agent src/ ./src/
COPY --chown=agent:agent config/ ./config/

# Switch to non-root user
USER agent

# Environment variables
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    ANTHROPIC_API_KEY="" \
    AGENT_PORT=8080 \
    AGENT_HOST="0.0.0.0" \
    LOG_LEVEL="info" \
    MAX_CONCURRENT_REQUESTS=100 \
    REQUEST_TIMEOUT=300 \
    HEALTH_CHECK_INTERVAL=30

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Graceful shutdown handler
STOPSIGNAL SIGTERM

# Start the agent server
CMD ["python", "-m", "src.agent_server"]
