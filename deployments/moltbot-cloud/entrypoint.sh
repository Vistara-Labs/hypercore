#!/bin/bash
# Clawdbot Cloud Entrypoint
# Fast startup by writing config directly instead of running multiple commands

set -e

echo "Clawdbot Cloud: Starting..."

# Generate a token if not provided (hex only for JSON safety)
GATEWAY_TOKEN="${CLAWDBOT_GATEWAY_TOKEN:-$(openssl rand -hex 16)}"
# Validate token is hex-safe (alphanumeric only, no special chars)
if [[ ! "$GATEWAY_TOKEN" =~ ^[a-zA-Z0-9_-]+$ ]]; then
  echo "WARNING: Token contains special characters, generating safe token"
  GATEWAY_TOKEN=$(openssl rand -hex 16)
fi

# Validate port is numeric
GATEWAY_PORT="${CLAWDBOT_PORT:-18789}"
if [[ ! "$GATEWAY_PORT" =~ ^[0-9]+$ ]]; then
  echo "WARNING: Invalid port '$GATEWAY_PORT', using default 18789"
  GATEWAY_PORT=18789
fi

# Create config directory
mkdir -p ~/.clawdbot

# Write config to clawdbot.json (the config file the gateway reads)
cat > ~/.clawdbot/clawdbot.json << EOF
{
  "gateway": {
    "port": ${GATEWAY_PORT},
    "mode": "local",
    "bind": "lan",
    "auth": {
      "mode": "token",
      "token": "${GATEWAY_TOKEN}"
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "anthropic/claude-sonnet-4-20250514"
      },
      "workspace": "/home/clawdbot/workspace"
    }
  }
}
EOF

# Create workspace directory
mkdir -p ~/workspace

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    CLAWDBOT CLOUD READY                        ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Token: $GATEWAY_TOKEN"
echo ""

# Check API keys and create auth-profiles.json
if [ -n "$ANTHROPIC_API_KEY" ]; then
  echo "ANTHROPIC_API_KEY is set (${#ANTHROPIC_API_KEY} chars)"

  # Create auth-profiles directory
  mkdir -p ~/.clawdbot/agents/main/agent

  # Write auth-profiles.json with proper JSON escaping
  if command -v jq &> /dev/null; then
    # Use jq for safe JSON generation
    jq -n \
      --arg key "$ANTHROPIC_API_KEY" \
      '{
        version: 1,
        profiles: {
          "anthropic:default": {
            type: "api_key",
            provider: "anthropic",
            key: $key
          }
        },
        lastGood: {
          anthropic: "anthropic:default"
        }
      }' > ~/.clawdbot/agents/main/agent/auth-profiles.json
  else
    # Fallback: validate key has no dangerous chars (Anthropic keys are base64-safe)
    if [[ "$ANTHROPIC_API_KEY" =~ ^[a-zA-Z0-9_-]+$ ]]; then
      cat > ~/.clawdbot/agents/main/agent/auth-profiles.json << AUTHEOF
{
  "version": 1,
  "profiles": {
    "anthropic:default": {
      "type": "api_key",
      "provider": "anthropic",
      "key": "${ANTHROPIC_API_KEY}"
    }
  },
  "lastGood": {
    "anthropic": "anthropic:default"
  }
}
AUTHEOF
    else
      echo "ERROR: API key contains invalid characters"
      exit 1
    fi
  fi
  echo "Created auth-profiles.json"
else
  echo "WARNING: ANTHROPIC_API_KEY is NOT set"
fi

# Start the gateway (clawdbot is the actual binary name)
# --allow-unconfigured is needed since we're writing config directly instead of using `clawdbot setup`
exec clawdbot gateway run --bind lan --port "${GATEWAY_PORT}" --allow-unconfigured --token "${GATEWAY_TOKEN}"
