"""Configuration management using pydantic-settings"""
import os
from typing import Optional
from pydantic_settings import BaseSettings
from pydantic import Field


class Settings(BaseSettings):
    """Application settings with environment variable support"""

    # API Configuration
    anthropic_api_key: str = Field(..., env="ANTHROPIC_API_KEY")
    agent_host: str = Field(default="0.0.0.0", env="AGENT_HOST")
    agent_port: int = Field(default=8080, env="AGENT_PORT")

    # Performance
    max_concurrent_requests: int = Field(default=100, env="MAX_CONCURRENT_REQUESTS")
    request_timeout: int = Field(default=300, env="REQUEST_TIMEOUT")
    worker_count: int = Field(default=4, env="WORKER_COUNT")

    # Logging
    log_level: str = Field(default="info", env="LOG_LEVEL")
    log_format: str = Field(default="json", env="LOG_FORMAT")

    # Redis (for session management in multi-instance setup)
    redis_enabled: bool = Field(default=False, env="REDIS_ENABLED")
    redis_host: str = Field(default="localhost", env="REDIS_HOST")
    redis_port: int = Field(default=6379, env="REDIS_PORT")
    redis_db: int = Field(default=0, env="REDIS_DB")
    redis_password: Optional[str] = Field(default=None, env="REDIS_PASSWORD")

    # Rate limiting
    rate_limit_per_minute: int = Field(default=100, env="RATE_LIMIT_PER_MINUTE")
    rate_limit_per_hour: int = Field(default=1000, env="RATE_LIMIT_PER_HOUR")

    # Monitoring
    metrics_port: int = Field(default=9090, env="METRICS_PORT")
    health_check_interval: int = Field(default=30, env="HEALTH_CHECK_INTERVAL")

    # Security
    enable_auth: bool = Field(default=False, env="ENABLE_AUTH")
    jwt_secret: Optional[str] = Field(default=None, env="JWT_SECRET")

    # Claude Agent SDK settings
    claude_model: str = Field(default="claude-3-5-sonnet-20241022", env="CLAUDE_MODEL")
    default_max_tokens: int = Field(default=4096, env="DEFAULT_MAX_TOKENS")
    default_temperature: float = Field(default=1.0, env="DEFAULT_TEMPERATURE")

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = False


# Global settings instance
settings = Settings()
