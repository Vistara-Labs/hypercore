"""Unit tests for AgentManager"""
import pytest
import asyncio
from unittest.mock import Mock, AsyncMock, patch
from datetime import datetime

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.agent_manager import AgentManager, AgentSession


class TestAgentSession:
    """Test AgentSession class"""

    def test_session_creation(self):
        """Test creating a session"""
        session = AgentSession("session-123", "user-456", "You are helpful")

        assert session.session_id == "session-123"
        assert session.user_id == "user-456"
        assert session.system_prompt == "You are helpful"
        assert len(session.message_history) == 0
        assert session.token_usage["total"] == 0

    def test_add_message(self):
        """Test adding messages to history"""
        session = AgentSession("session-123", "user-456")

        session.add_message("user", "Hello")
        session.add_message("assistant", "Hi there!")

        assert len(session.message_history) == 2
        assert session.message_history[0]["role"] == "user"
        assert session.message_history[1]["content"] == "Hi there!"

    def test_update_token_usage(self):
        """Test token usage tracking"""
        session = AgentSession("session-123", "user-456")

        session.update_token_usage({"input_tokens": 100, "output_tokens": 50})

        assert session.token_usage["input"] == 100
        assert session.token_usage["output"] == 50
        assert session.token_usage["total"] == 150

    def test_session_expiry(self):
        """Test session expiration check"""
        session = AgentSession("session-123", "user-456")

        # Not expired with 300s timeout
        assert not session.is_expired(300)

        # Force expiry
        session.last_activity = datetime.now().timestamp() - 400
        assert session.is_expired(300)


class TestAgentManager:
    """Test AgentManager class"""

    @pytest.fixture
    def mock_config(self):
        """Mock configuration"""
        with patch('src.agent_manager.settings') as mock_settings:
            mock_settings.anthropic_api_key = "test-key"
            mock_settings.claude_model = "claude-3-5-sonnet-20241022"
            yield mock_settings

    @pytest.fixture
    def agent_manager(self, mock_config):
        """Create AgentManager instance"""
        with patch('src.agent_manager.AsyncAnthropic'):
            manager = AgentManager(max_concurrent_sessions=10, session_timeout=300)
            return manager

    @pytest.mark.asyncio
    async def test_create_session(self, agent_manager):
        """Test session creation"""
        session_id = await agent_manager.create_session("user-123", "Be helpful")

        assert session_id in agent_manager.sessions
        assert agent_manager.sessions[session_id].user_id == "user-123"
        assert agent_manager.stats["total_sessions"] == 1

    @pytest.mark.asyncio
    async def test_create_session_limit(self, agent_manager):
        """Test session limit enforcement"""
        # Fill up to max
        for i in range(10):
            await agent_manager.create_session(f"user-{i}")

        # Should fail on 11th
        with pytest.raises(Exception, match="Maximum concurrent sessions reached"):
            await agent_manager.create_session("user-11")

    @pytest.mark.asyncio
    async def test_delete_session(self, agent_manager):
        """Test session deletion"""
        session_id = await agent_manager.create_session("user-123")

        await agent_manager.delete_session(session_id)

        assert session_id not in agent_manager.sessions
        assert agent_manager.stats["active_sessions"] == 0

    @pytest.mark.asyncio
    async def test_process_request_session_not_found(self, agent_manager):
        """Test processing request with invalid session"""
        mock_request = Mock(prompt="Hello")

        with pytest.raises(Exception, match="Session not found"):
            await agent_manager.process_request("invalid-session", mock_request)

    @pytest.mark.asyncio
    async def test_process_request_success(self, agent_manager):
        """Test successful request processing"""
        # Mock Anthropic client response
        mock_response = Mock()
        mock_response.content = [Mock(text="Hello! How can I help?")]
        mock_response.usage = Mock(input_tokens=10, output_tokens=20)

        agent_manager.client.messages.create = AsyncMock(return_value=mock_response)

        # Create session and make request
        session_id = await agent_manager.create_session("user-123")
        mock_request = Mock(
            prompt="Hello",
            max_tokens=100,
            temperature=1.0,
            system_prompt=None,
            tools=[]
        )

        result = await agent_manager.process_request(session_id, mock_request)

        assert result["content"] == "Hello! How can I help?"
        assert result["usage"]["input_tokens"] == 10
        assert result["usage"]["output_tokens"] == 20
        assert result["usage"]["total_tokens"] == 30

    @pytest.mark.asyncio
    async def test_cleanup_expired_sessions(self, agent_manager):
        """Test expired session cleanup"""
        # Create sessions
        session1 = await agent_manager.create_session("user-1")
        session2 = await agent_manager.create_session("user-2")

        # Expire session1
        agent_manager.sessions[session1].last_activity = datetime.now().timestamp() - 400

        # Run cleanup
        await agent_manager._cleanup_expired_sessions_sync()

        assert session1 not in agent_manager.sessions
        assert session2 in agent_manager.sessions

    @pytest.mark.asyncio
    async def test_get_stats(self, agent_manager):
        """Test statistics retrieval"""
        await agent_manager.create_session("user-1")
        await agent_manager.create_session("user-2")

        stats = await agent_manager.get_stats()

        assert stats["total_sessions"] == 2
        assert stats["active_sessions"] == 2
        assert stats["total_requests"] == 0

    @pytest.mark.asyncio
    async def test_is_ready(self, agent_manager):
        """Test readiness check"""
        assert await agent_manager.is_ready() == True

        agent_manager.is_shutting_down = True
        assert await agent_manager.is_ready() == False

    @pytest.mark.asyncio
    async def test_shutdown(self, agent_manager):
        """Test graceful shutdown"""
        # Create some sessions
        await agent_manager.create_session("user-1")
        await agent_manager.create_session("user-2")

        await agent_manager.shutdown()

        assert len(agent_manager.sessions) == 0
        assert agent_manager.is_shutting_down == True


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
