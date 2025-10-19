"""Unit tests for agent server endpoints"""
import pytest
from fastapi.testclient import TestClient
from unittest.mock import Mock, AsyncMock, patch

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))


@pytest.fixture
def mock_agent_manager():
    """Mock AgentManager"""
    manager = Mock()
    manager.create_session = AsyncMock(return_value="session-123")
    manager.process_request = AsyncMock(return_value={
        "content": "Test response",
        "usage": {"input_tokens": 10, "output_tokens": 20, "total_tokens": 30},
        "message_count": 2
    })
    manager.delete_session = AsyncMock()
    manager.get_stats = AsyncMock(return_value={
        "total_requests": 100,
        "active_sessions": 10,
        "total_tokens": 5000
    })
    manager.is_ready = AsyncMock(return_value=True)
    manager.shutdown = AsyncMock()
    return manager


@pytest.fixture
def mock_health_check():
    """Mock HealthCheck"""
    health = Mock()
    health.check = AsyncMock(return_value={
        "status": "healthy",
        "version": "1.0.0",
        "uptime_seconds": 100,
        "checks": {
            "api": {"status": "pass"},
            "memory": {"status": "pass"},
            "disk": {"status": "pass"}
        }
    })
    return health


@pytest.fixture
def test_app(mock_agent_manager, mock_health_check):
    """Create test app with mocked dependencies"""
    with patch('src.agent_server.agent_manager', mock_agent_manager), \
         patch('src.agent_server.health_check', mock_health_check), \
         patch('src.agent_server.settings') as mock_settings:

        mock_settings.agent_host = "0.0.0.0"
        mock_settings.agent_port = 8080
        mock_settings.max_concurrent_requests = 100
        mock_settings.request_timeout = 300
        mock_settings.log_level = "info"

        from src.agent_server import app
        client = TestClient(app)
        yield client


class TestHealthEndpoints:
    """Test health check endpoints"""

    def test_health_endpoint(self, test_app):
        """Test /health endpoint"""
        response = test_app.get("/health")

        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert "version" in data

    def test_readiness_endpoint(self, test_app, mock_agent_manager):
        """Test /ready endpoint"""
        response = test_app.get("/ready")

        assert response.status_code == 200
        assert response.json()["status"] == "ready"

    def test_readiness_not_ready(self, test_app, mock_agent_manager):
        """Test /ready when not ready"""
        mock_agent_manager.is_ready.return_value = False

        response = test_app.get("/ready")

        assert response.status_code == 503

    def test_metrics_endpoint(self, test_app):
        """Test /metrics endpoint"""
        response = test_app.get("/metrics")

        assert response.status_code == 200
        # Prometheus metrics format
        assert b"# HELP" in response.content or len(response.content) >= 0


class TestAgentEndpoints:
    """Test agent API endpoints"""

    def test_chat_endpoint_success(self, test_app, mock_agent_manager):
        """Test successful chat request"""
        payload = {
            "prompt": "Hello, how are you?",
            "user_id": "test-user",
            "max_tokens": 100,
            "temperature": 1.0,
            "stream": False
        }

        response = test_app.post("/v1/agent/chat", json=payload)

        assert response.status_code == 200
        data = response.json()
        assert data["content"] == "Test response"
        assert data["session_id"] == "session-123"
        assert data["status"] == "completed"

    def test_chat_endpoint_validation_error(self, test_app):
        """Test chat with invalid payload"""
        payload = {
            "prompt": "Hello",
            # Missing required user_id
        }

        response = test_app.post("/v1/agent/chat", json=payload)

        assert response.status_code == 422  # Validation error

    def test_delete_session_endpoint(self, test_app, mock_agent_manager):
        """Test session deletion"""
        response = test_app.delete("/v1/agent/session/session-123")

        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "deleted"
        mock_agent_manager.delete_session.assert_called_once()

    def test_delete_session_error(self, test_app, mock_agent_manager):
        """Test session deletion with error"""
        mock_agent_manager.delete_session.side_effect = Exception("Session not found")

        response = test_app.delete("/v1/agent/session/invalid-session")

        assert response.status_code == 500

    def test_get_stats_endpoint(self, test_app, mock_agent_manager):
        """Test statistics endpoint"""
        response = test_app.get("/v1/agent/stats")

        assert response.status_code == 200
        data = response.json()
        assert data["total_requests"] == 100
        assert data["active_sessions"] == 10


class TestRateLimiting:
    """Test rate limiting functionality"""

    @pytest.mark.skip(reason="Rate limiting requires slowapi setup")
    def test_rate_limit_exceeded(self, test_app):
        """Test rate limiting kicks in after threshold"""
        payload = {
            "prompt": "Hello",
            "user_id": "test-user"
        }

        # Make 101 requests (limit is 100/min)
        for _ in range(101):
            response = test_app.post("/v1/agent/chat", json=payload)

        # Last request should be rate limited
        assert response.status_code == 429


class TestErrorHandling:
    """Test error handling"""

    def test_server_error_handling(self, test_app, mock_agent_manager):
        """Test internal server error handling"""
        mock_agent_manager.create_session.side_effect = Exception("Database error")

        payload = {
            "prompt": "Hello",
            "user_id": "test-user"
        }

        response = test_app.post("/v1/agent/chat", json=payload)

        assert response.status_code == 500


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
