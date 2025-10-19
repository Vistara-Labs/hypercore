"""Metrics setup and configuration"""
import structlog
from prometheus_client import CollectorRegistry, Counter, Histogram, Gauge, Info

logger = structlog.get_logger()

# Custom registry for better control
registry = CollectorRegistry()


def setup_metrics():
    """Setup Prometheus metrics"""

    # Application info
    app_info = Info('agent_server', 'Claude Agent Server info', registry=registry)
    app_info.info({
        'version': '1.0.0',
        'model': 'claude-3-5-sonnet-20241022'
    })

    logger.info("Metrics configured successfully")

    return registry
