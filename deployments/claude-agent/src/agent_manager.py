"""
Agent Manager - Handles agent lifecycle and session management
Optimized for 10k+ concurrent users
"""
import asyncio
import time
import uuid
from typing import Dict, Optional, AsyncGenerator
from collections import defaultdict

import structlog
from anthropic import AsyncAnthropic
from tenacity import retry, stop_after_attempt, wait_exponential

from .config import settings

logger = structlog.get_logger()


class AgentSession:
    """Represents an individual agent session"""

    def __init__(self, session_id: str, user_id: str, system_prompt: Optional[str] = None):
        self.session_id = session_id
        self.user_id = user_id
        self.system_prompt = system_prompt
        self.created_at = time.time()
        self.last_activity = time.time()
        self.message_history = []
        self.token_usage = {"input": 0, "output": 0, "total": 0}

    def update_activity(self):
        """Update last activity timestamp"""
        self.last_activity = time.time()

    def add_message(self, role: str, content: str):
        """Add message to history"""
        self.message_history.append({"role": role, "content": content})
        self.update_activity()

    def update_token_usage(self, usage: dict):
        """Update token usage statistics"""
        self.token_usage["input"] += usage.get("input_tokens", 0)
        self.token_usage["output"] += usage.get("output_tokens", 0)
        self.token_usage["total"] = self.token_usage["input"] + self.token_usage["output"]

    def is_expired(self, timeout: int) -> bool:
        """Check if session has expired"""
        return (time.time() - self.last_activity) > timeout


class AgentManager:
    """
    Manages agent lifecycle, sessions, and Claude API interactions
    Designed for high concurrency and reliability
    """

    def __init__(self, max_concurrent_sessions: int = 100, session_timeout: int = 300):
        self.client = AsyncAnthropic(api_key=settings.anthropic_api_key)
        self.sessions: Dict[str, AgentSession] = {}
        self.max_concurrent_sessions = max_concurrent_sessions
        self.session_timeout = session_timeout
        self.session_lock = asyncio.Lock()
        self.cleanup_task = None
        self.is_shutting_down = False

        # Statistics
        self.stats = {
            "total_requests": 0,
            "total_tokens": 0,
            "total_sessions": 0,
            "active_sessions": 0,
            "errors": defaultdict(int)
        }

        # Start background cleanup task
        self.cleanup_task = asyncio.create_task(self._cleanup_expired_sessions())

        logger.info("AgentManager initialized",
                   max_sessions=max_concurrent_sessions,
                   timeout=session_timeout)

    async def create_session(self, user_id: str, system_prompt: Optional[str] = None) -> str:
        """Create a new agent session"""
        async with self.session_lock:
            # Check concurrent session limit
            if len(self.sessions) >= self.max_concurrent_sessions:
                # Clean up old sessions first
                await self._cleanup_expired_sessions_sync()

                if len(self.sessions) >= self.max_concurrent_sessions:
                    raise Exception(f"Maximum concurrent sessions reached: {self.max_concurrent_sessions}")

            session_id = str(uuid.uuid4())
            session = AgentSession(session_id, user_id, system_prompt)
            self.sessions[session_id] = session

            self.stats["total_sessions"] += 1
            self.stats["active_sessions"] = len(self.sessions)

            logger.info("Session created",
                       session_id=session_id,
                       user_id=user_id,
                       active_sessions=len(self.sessions))

            return session_id

    @retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=2, max=10))
    async def process_request(self, session_id: str, request) -> dict:
        """
        Process an agent request with retry logic
        """
        session = self.sessions.get(session_id)
        if not session:
            raise Exception(f"Session not found: {session_id}")

        try:
            # Add user message to history
            session.add_message("user", request.prompt)

            # Prepare messages
            messages = session.message_history.copy()

            # Call Claude API
            response = await self.client.messages.create(
                model=settings.claude_model,
                max_tokens=request.max_tokens,
                temperature=request.temperature,
                system=request.system_prompt or session.system_prompt or "You are a helpful AI assistant.",
                messages=messages,
                tools=request.tools or []
            )

            # Extract content
            content = ""
            for block in response.content:
                if hasattr(block, 'text'):
                    content += block.text

            # Add assistant response to history
            session.add_message("assistant", content)

            # Update token usage
            usage = {
                "input_tokens": response.usage.input_tokens,
                "output_tokens": response.usage.output_tokens,
                "total_tokens": response.usage.input_tokens + response.usage.output_tokens
            }
            session.update_token_usage(usage)

            # Update stats
            self.stats["total_requests"] += 1
            self.stats["total_tokens"] += usage["total_tokens"]

            logger.info("Request processed",
                       session_id=session_id,
                       tokens=usage["total_tokens"],
                       messages=len(session.message_history))

            return {
                "content": content,
                "usage": usage,
                "message_count": len(session.message_history)
            }

        except Exception as e:
            self.stats["errors"][type(e).__name__] += 1
            logger.error("Request processing error",
                        session_id=session_id,
                        error=str(e),
                        error_type=type(e).__name__)
            raise

    async def stream_response(self, session_id: str, request) -> AsyncGenerator[str, None]:
        """
        Stream agent response for real-time interaction
        """
        session = self.sessions.get(session_id)
        if not session:
            raise Exception(f"Session not found: {session_id}")

        try:
            session.add_message("user", request.prompt)
            messages = session.message_history.copy()

            accumulated_content = ""

            async with self.client.messages.stream(
                model=settings.claude_model,
                max_tokens=request.max_tokens,
                temperature=request.temperature,
                system=request.system_prompt or session.system_prompt or "You are a helpful AI assistant.",
                messages=messages,
            ) as stream:
                async for text in stream.text_stream:
                    accumulated_content += text
                    yield f"data: {text}\n\n"

            # Add complete response to history
            session.add_message("assistant", accumulated_content)

            # Get final usage
            message = await stream.get_final_message()
            usage = {
                "input_tokens": message.usage.input_tokens,
                "output_tokens": message.usage.output_tokens,
                "total_tokens": message.usage.input_tokens + message.usage.output_tokens
            }
            session.update_token_usage(usage)

            yield f"data: [DONE]\n\n"

        except Exception as e:
            self.stats["errors"][type(e).__name__] += 1
            logger.error("Streaming error", session_id=session_id, error=str(e))
            yield f"data: {{\"error\": \"{str(e)}\"}}\n\n"

    async def delete_session(self, session_id: str):
        """Delete an agent session"""
        async with self.session_lock:
            if session_id in self.sessions:
                del self.sessions[session_id]
                self.stats["active_sessions"] = len(self.sessions)
                logger.info("Session deleted", session_id=session_id)

    async def _cleanup_expired_sessions_sync(self):
        """Cleanup expired sessions (synchronous version for internal use)"""
        now = time.time()
        expired = [
            sid for sid, session in self.sessions.items()
            if session.is_expired(self.session_timeout)
        ]

        for session_id in expired:
            del self.sessions[session_id]
            logger.debug("Expired session cleaned up", session_id=session_id)

        if expired:
            self.stats["active_sessions"] = len(self.sessions)
            logger.info("Cleanup completed", expired_count=len(expired), active=len(self.sessions))

    async def _cleanup_expired_sessions(self):
        """Background task to cleanup expired sessions"""
        while not self.is_shutting_down:
            try:
                await asyncio.sleep(60)  # Run every minute
                async with self.session_lock:
                    await self._cleanup_expired_sessions_sync()
            except Exception as e:
                logger.error("Cleanup task error", error=str(e))

    async def get_stats(self) -> dict:
        """Get agent statistics"""
        return {
            "total_requests": self.stats["total_requests"],
            "total_tokens": self.stats["total_tokens"],
            "total_sessions": self.stats["total_sessions"],
            "active_sessions": len(self.sessions),
            "errors": dict(self.stats["errors"]),
            "uptime_seconds": time.time() - (self.sessions[list(self.sessions.keys())[0]].created_at if self.sessions else time.time())
        }

    async def is_ready(self) -> bool:
        """Check if agent manager is ready to accept requests"""
        return not self.is_shutting_down and self.client is not None

    async def shutdown(self):
        """Graceful shutdown"""
        logger.info("Starting graceful shutdown...")
        self.is_shutting_down = True

        # Cancel cleanup task
        if self.cleanup_task:
            self.cleanup_task.cancel()
            try:
                await self.cleanup_task
            except asyncio.CancelledError:
                pass

        # Close all sessions
        async with self.session_lock:
            session_count = len(self.sessions)
            self.sessions.clear()
            logger.info("All sessions closed", count=session_count)

        logger.info("Shutdown complete")
