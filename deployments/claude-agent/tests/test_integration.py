"""Integration tests for full agent workflow"""
import pytest
import asyncio
from unittest.mock import Mock, AsyncMock, patch
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from src.agent_manager import AgentManager


@pytest.mark.integration
class TestAgentWorkflow:
    """Test complete agent workflow"""

    @pytest.fixture
    async def agent_manager_integration(self):
        """Create AgentManager with mocked Anthropic client"""
        with patch('src.agent_manager.AsyncAnthropic') as mock_anthropic, \
             patch('src.agent_manager.settings') as mock_settings:

            mock_settings.anthropic_api_key = "test-key"
            mock_settings.claude_model = "claude-3-5-sonnet-20241022"

            manager = AgentManager(max_concurrent_sessions=10, session_timeout=300)

            # Mock Anthropic responses
            mock_response = Mock()
            mock_response.content = [Mock(text="I'm doing well, thank you!")]
            mock_response.usage = Mock(input_tokens=10, output_tokens=15)

            manager.client.messages.create = AsyncMock(return_value=mock_response)

            yield manager

            await manager.shutdown()

    @pytest.mark.asyncio
    async def test_complete_conversation_flow(self, agent_manager_integration):
        """Test complete conversation workflow"""
        manager = agent_manager_integration

        # 1. Create session
        session_id = await manager.create_session("user-123", "You are a helpful assistant")
        assert session_id in manager.sessions

        # 2. First message
        request1 = Mock(
            prompt="Hello, how are you?",
            max_tokens=100,
            temperature=1.0,
            system_prompt=None,
            tools=[]
        )
        response1 = await manager.process_request(session_id, request1)

        assert "doing well" in response1["content"].lower()
        assert response1["usage"]["total_tokens"] == 25

        # 3. Second message in same session
        request2 = Mock(
            prompt="What's the weather?",
            max_tokens=100,
            temperature=1.0,
            system_prompt=None,
            tools=[]
        )
        response2 = await manager.process_request(session_id, request2)

        # Verify session maintains history
        session = manager.sessions[session_id]
        assert len(session.message_history) == 4  # 2 user + 2 assistant
        assert session.token_usage["total"] == 50  # 25 + 25

        # 4. Delete session
        await manager.delete_session(session_id)
        assert session_id not in manager.sessions

    @pytest.mark.asyncio
    async def test_concurrent_sessions(self, agent_manager_integration):
        """Test multiple concurrent sessions"""
        manager = agent_manager_integration

        # Create multiple sessions
        sessions = []
        for i in range(5):
            session_id = await manager.create_session(f"user-{i}")
            sessions.append(session_id)

        assert len(manager.sessions) == 5

        # Process requests concurrently
        async def make_request(session_id):
            request = Mock(
                prompt="Hello",
                max_tokens=50,
                temperature=1.0,
                system_prompt=None,
                tools=[]
            )
            return await manager.process_request(session_id, request)

        results = await asyncio.gather(*[make_request(sid) for sid in sessions])

        assert len(results) == 5
        assert all(r["content"] for r in results)

        # Cleanup
        for session_id in sessions:
            await manager.delete_session(session_id)

    @pytest.mark.asyncio
    async def test_session_limit_enforcement(self, agent_manager_integration):
        """Test that session limits are enforced"""
        manager = agent_manager_integration

        # Fill to capacity
        sessions = []
        for i in range(10):
            session_id = await manager.create_session(f"user-{i}")
            sessions.append(session_id)

        # Try to create 11th session
        with pytest.raises(Exception, match="Maximum concurrent sessions reached"):
            await manager.create_session("user-overflow")

        # Cleanup one session
        await manager.delete_session(sessions[0])

        # Now should work
        new_session = await manager.create_session("user-new")
        assert new_session in manager.sessions

    @pytest.mark.asyncio
    async def test_token_accumulation(self, agent_manager_integration):
        """Test token usage accumulation over multiple requests"""
        manager = agent_manager_integration

        session_id = await manager.create_session("user-123")

        # Make multiple requests
        for i in range(3):
            request = Mock(
                prompt=f"Question {i}",
                max_tokens=100,
                temperature=1.0,
                system_prompt=None,
                tools=[]
            )
            await manager.process_request(session_id, request)

        # Check accumulated tokens
        session = manager.sessions[session_id]
        assert session.token_usage["total"] == 75  # 25 per request × 3

    @pytest.mark.asyncio
    async def test_error_recovery(self, agent_manager_integration):
        """Test error handling and recovery"""
        manager = agent_manager_integration

        session_id = await manager.create_session("user-123")

        # Simulate API error
        manager.client.messages.create = AsyncMock(
            side_effect=Exception("API Error")
        )

        request = Mock(
            prompt="Hello",
            max_tokens=100,
            temperature=1.0,
            system_prompt=None,
            tools=[]
        )

        # Should raise error
        with pytest.raises(Exception, match="API Error"):
            await manager.process_request(session_id, request)

        # Session should still exist
        assert session_id in manager.sessions

        # Fix the error
        mock_response = Mock()
        mock_response.content = [Mock(text="Recovered!")]
        mock_response.usage = Mock(input_tokens=5, output_tokens=5)
        manager.client.messages.create = AsyncMock(return_value=mock_response)

        # Should work now
        response = await manager.process_request(session_id, request)
        assert response["content"] == "Recovered!"


if __name__ == "__main__":
    pytest.main([__file__, "-v", "-m", "integration"])
