# Clawdbot VM Image (Built from Source)
# Full Clawdbot gateway running in Hypercore microVMs
# Usage: hypercore spawn clawdbot/clawdbot-vm:latest

FROM node:22-bookworm AS builder

# Install pnpm
RUN npm install -g pnpm

# Copy source code
WORKDIR /build
COPY . .

# Install and build (including UI)
RUN pnpm install --frozen-lockfile
RUN pnpm build
RUN pnpm ui:build

# Create tarball for global install
RUN pnpm pack

# Runtime image - use node base for native module compatibility
FROM node:22-bookworm-slim

# Install base packages and build tools for native modules
RUN apt-get update && apt-get install -y \
    curl \
    git \
    unzip \
    ca-certificates \
    sudo \
    build-essential \
    python3 \
    && rm -rf /var/lib/apt/lists/*

# Copy and install built clawdbot
COPY --from=builder /build/*.tgz /tmp/clawdbot.tgz
RUN npm install -g /tmp/clawdbot.tgz && rm /tmp/clawdbot.tgz

# Create clawdbot user
RUN useradd -m -s /bin/bash clawdbot && \
    echo "clawdbot ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Switch to clawdbot user
USER clawdbot
WORKDIR /home/clawdbot
ENV HOME=/home/clawdbot

# Create config directory
RUN mkdir -p /home/clawdbot/.clawdbot

# Copy entrypoint script
COPY --chown=clawdbot:clawdbot images/clawdbot-vm/entrypoint.sh /opt/entrypoint.sh
RUN chmod +x /opt/entrypoint.sh

# Expose gateway port
EXPOSE 18789

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:18789/health || exit 1

# Default environment
ENV CLAWDBOT_PROVIDER=anthropic
ENV CLAWDBOT_MODEL=claude-sonnet-4-20250514
ENV CLAWDBOT_PORT=18789

# Start Clawdbot gateway via entrypoint
CMD ["/opt/entrypoint.sh"]
