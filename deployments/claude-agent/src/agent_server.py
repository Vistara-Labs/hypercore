"""
Claude Agent SDK Server
Production-ready FastAPI server for Claude Agent SDK integration
Designed for 10k+ concurrent users
"""
import asyncio
import signal
import sys
from contextlib import asynccontextmanager
from typing import Optional

import structlog
from fastapi import FastAPI, HTTPException, Request, BackgroundTasks
from fastapi.responses import JSONResponse, StreamingResponse
from prometheus_client import Counter, Histogram, Gauge, generate_latest
from anthropic import Anthropic, AsyncAnthropic
from pydantic import BaseModel, Field
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.util import get_remote_address
from slowapi.errors import RateLimitExceeded

from .config import settings
from .metrics import setup_metrics
from .agent_manager import AgentManager
from .health import HealthCheck

# Structured logging
logger = structlog.get_logger()

# Prometheus metrics
REQUESTS_TOTAL = Counter('agent_requests_total', 'Total agent requests', ['endpoint', 'status'])
REQUEST_DURATION = Histogram('agent_request_duration_seconds', 'Request duration', ['endpoint'])
ACTIVE_SESSIONS = Gauge('agent_active_sessions', 'Active agent sessions')
TOKEN_USAGE = Counter('agent_token_usage_total', 'Total tokens used', ['type'])
ERRORS_TOTAL = Counter('agent_errors_total', 'Total errors', ['error_type'])

# Rate limiter
limiter = Limiter(key_func=get_remote_address)

# Global agent manager
agent_manager: Optional[AgentManager] = None
health_check: Optional[HealthCheck] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifecycle management for the application"""
    global agent_manager, health_check

    logger.info("Starting Claude Agent Server", version="1.0.0", max_users=10000)

    # Initialize components
    agent_manager = AgentManager(
        max_concurrent_sessions=settings.max_concurrent_requests,
        session_timeout=settings.request_timeout
    )
    health_check = HealthCheck()

    # Setup graceful shutdown
    def signal_handler(signum, frame):
        logger.info("Received shutdown signal", signal=signum)
        asyncio.create_task(agent_manager.shutdown())

    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    logger.info("Server started successfully")
    yield

    # Cleanup
    logger.info("Shutting down gracefully...")
    await agent_manager.shutdown()
    logger.info("Shutdown complete")


# Create FastAPI app
app = FastAPI(
    title="Claude Agent SDK Server",
    description="Production-ready Claude Agent server for hypercore",
    version="1.0.0",
    lifespan=lifespan
)

app.state.limiter = limiter
app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)


# Request/Response models
class AgentRequest(BaseModel):
    """Agent request payload"""
    prompt: str = Field(..., description="User prompt for the agent")
    user_id: str = Field(..., description="User ID for session management")
    max_tokens: int = Field(default=4096, ge=1, le=8192)
    temperature: float = Field(default=1.0, ge=0.0, le=2.0)
    stream: bool = Field(default=False, description="Enable streaming response")
    tools: Optional[list] = Field(default=None, description="Available tools")
    system_prompt: Optional[str] = Field(default=None)


class AgentResponse(BaseModel):
    """Agent response payload"""
    session_id: str
    content: str
    usage: dict
    status: str


class HealthResponse(BaseModel):
    """Health check response"""
    status: str
    version: str
    active_sessions: int
    uptime_seconds: float


# API Endpoints
@app.get("/health")
async def health():
    """Health check endpoint"""
    try:
        health_status = await health_check.check()
        return JSONResponse(
            content=health_status,
            status_code=200 if health_status["status"] == "healthy" else 503
        )
    except Exception as e:
        logger.error("Health check failed", error=str(e))
        return JSONResponse(
            content={"status": "unhealthy", "error": str(e)},
            status_code=503
        )


@app.get("/ready")
async def readiness():
    """Readiness check for k8s/orchestration"""
    if agent_manager is None:
        raise HTTPException(status_code=503, detail="Agent manager not initialized")

    ready = await agent_manager.is_ready()
    if not ready:
        raise HTTPException(status_code=503, detail="Service not ready")

    return {"status": "ready"}


@app.get("/metrics")
async def metrics():
    """Prometheus metrics endpoint"""
    return generate_latest()


@app.post("/v1/agent/chat", response_model=AgentResponse)
@limiter.limit("100/minute")
async def chat(request: Request, agent_request: AgentRequest, background_tasks: BackgroundTasks):
    """
    Main agent chat endpoint
    Rate limited to 100 requests/minute per IP
    """
    session_id = None
    start_time = asyncio.get_event_loop().time()

    try:
        REQUESTS_TOTAL.labels(endpoint='chat', status='started').inc()
        ACTIVE_SESSIONS.inc()

        logger.info("Agent chat request",
                   user_id=agent_request.user_id,
                   prompt_length=len(agent_request.prompt))

        # Create agent session
        session_id = await agent_manager.create_session(
            user_id=agent_request.user_id,
            system_prompt=agent_request.system_prompt
        )

        # Process request
        if agent_request.stream:
            # Streaming response
            return StreamingResponse(
                agent_manager.stream_response(session_id, agent_request),
                media_type="text/event-stream"
            )
        else:
            # Standard response
            response = await agent_manager.process_request(session_id, agent_request)

            # Track metrics
            TOKEN_USAGE.labels(type='input').inc(response['usage'].get('input_tokens', 0))
            TOKEN_USAGE.labels(type='output').inc(response['usage'].get('output_tokens', 0))

            duration = asyncio.get_event_loop().time() - start_time
            REQUEST_DURATION.labels(endpoint='chat').observe(duration)
            REQUESTS_TOTAL.labels(endpoint='chat', status='success').inc()

            logger.info("Agent chat completed",
                       session_id=session_id,
                       duration_ms=duration * 1000,
                       tokens=response['usage'].get('total_tokens', 0))

            return AgentResponse(
                session_id=session_id,
                content=response['content'],
                usage=response['usage'],
                status='completed'
            )

    except Exception as e:
        ERRORS_TOTAL.labels(error_type=type(e).__name__).inc()
        REQUESTS_TOTAL.labels(endpoint='chat', status='error').inc()
        logger.error("Agent chat error",
                    session_id=session_id,
                    error=str(e),
                    error_type=type(e).__name__)
        raise HTTPException(status_code=500, detail=str(e))

    finally:
        ACTIVE_SESSIONS.dec()


@app.delete("/v1/agent/session/{session_id}")
async def delete_session(session_id: str):
    """Delete an agent session"""
    try:
        await agent_manager.delete_session(session_id)
        logger.info("Session deleted", session_id=session_id)
        return {"status": "deleted", "session_id": session_id}
    except Exception as e:
        logger.error("Session deletion error", session_id=session_id, error=str(e))
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/v1/agent/stats")
async def get_stats():
    """Get agent statistics"""
    try:
        stats = await agent_manager.get_stats()
        return stats
    except Exception as e:
        logger.error("Stats retrieval error", error=str(e))
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn

    # Production-ready uvicorn configuration
    uvicorn.run(
        "agent_server:app",
        host=settings.agent_host,
        port=settings.agent_port,
        workers=4,  # Multi-process for CPU-bound tasks
        loop="uvloop",  # High-performance event loop
        http="httptools",  # Fast HTTP parser
        log_level=settings.log_level,
        access_log=True,
        timeout_keep_alive=75,
        limit_concurrency=settings.max_concurrent_requests,
        backlog=2048
    )
