"""Unit tests for configuration"""
import pytest
import os
from unittest.mock import patch

import sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))


class TestSettings:
    """Test configuration settings"""

    def test_default_settings(self):
        """Test default configuration values"""
        with patch.dict(os.environ, {"ANTHROPIC_API_KEY": "test-key"}):
            from src.config import Settings
            settings = Settings()

            assert settings.agent_host == "0.0.0.0"
            assert settings.agent_port == 8080
            assert settings.max_concurrent_requests == 100
            assert settings.request_timeout == 300
            assert settings.log_level == "info"

    def test_environment_override(self):
        """Test environment variable overrides"""
        env = {
            "ANTHROPIC_API_KEY": "test-key",
            "AGENT_PORT": "9000",
            "MAX_CONCURRENT_REQUESTS": "200",
            "LOG_LEVEL": "debug"
        }

        with patch.dict(os.environ, env):
            from src.config import Settings
            settings = Settings()

            assert settings.agent_port == 9000
            assert settings.max_concurrent_requests == 200
            assert settings.log_level == "debug"

    def test_redis_disabled_by_default(self):
        """Test Redis is disabled by default"""
        with patch.dict(os.environ, {"ANTHROPIC_API_KEY": "test-key"}):
            from src.config import Settings
            settings = Settings()

            assert settings.redis_enabled == False

    def test_redis_configuration(self):
        """Test Redis configuration when enabled"""
        env = {
            "ANTHROPIC_API_KEY": "test-key",
            "REDIS_ENABLED": "true",
            "REDIS_HOST": "redis.example.com",
            "REDIS_PORT": "6380",
            "REDIS_PASSWORD": "secret"
        }

        with patch.dict(os.environ, env):
            from src.config import Settings
            settings = Settings()

            assert settings.redis_enabled == True
            assert settings.redis_host == "redis.example.com"
            assert settings.redis_port == 6380
            assert settings.redis_password == "secret"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
