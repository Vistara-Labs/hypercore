#!/bin/bash
# Clawdbot Cloud Entrypoint
# Fast startup by writing config directly instead of running multiple commands

set -e

echo "Clawdbot Cloud: Starting..."

# Generate a token if not provided
GATEWAY_TOKEN="${CLAWDBOT_GATEWAY_TOKEN:-$(openssl rand -hex 16)}"
GATEWAY_PORT="${CLAWDBOT_PORT:-18789}"

# Create config directory
mkdir -p ~/.clawdbot

# Write config to moltbot.json (the config file the gateway reads)
cat > ~/.clawdbot/moltbot.json << EOF
{
  "gateway": {
    "port": ${GATEWAY_PORT},
    "mode": "local",
    "bind": "lan",
    "auth": {
      "mode": "token",
      "token": "${GATEWAY_TOKEN}"
    },
    "controlUi": {
      "dangerouslyDisableDeviceAuth": true
    }
  },
  "agents": {
    "defaults": {
      "workspace": "/home/clawdbot/workspace"
    }
  }
}
EOF

# Create workspace directory
mkdir -p ~/workspace

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    MOLTBOT CLOUD READY                        ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Token: $GATEWAY_TOKEN"
echo ""

# Check API keys
if [ -n "$ANTHROPIC_API_KEY" ]; then
  echo "ANTHROPIC_API_KEY is set (${#ANTHROPIC_API_KEY} chars)"
else
  echo "WARNING: ANTHROPIC_API_KEY is NOT set"
fi

# Start the gateway
exec moltbot gateway run --bind lan --port "${GATEWAY_PORT}"
