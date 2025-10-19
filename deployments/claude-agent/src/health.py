"""Health check implementation"""
import time
import asyncio
import structlog
from typing import Dict

from .config import settings

logger = structlog.get_logger()


class HealthCheck:
    """Health check system for monitoring service health"""

    def __init__(self):
        self.start_time = time.time()
        self.last_check = time.time()
        self.check_count = 0

    async def check(self) -> Dict:
        """Perform health check"""
        self.check_count += 1
        self.last_check = time.time()

        uptime = time.time() - self.start_time

        health_status = {
            "status": "healthy",
            "version": "1.0.0",
            "uptime_seconds": uptime,
            "checks": {
                "api": await self._check_api(),
                "memory": await self._check_memory(),
                "disk": await self._check_disk()
            }
        }

        # Determine overall health
        all_healthy = all(check["status"] == "pass" for check in health_status["checks"].values())
        health_status["status"] = "healthy" if all_healthy else "degraded"

        return health_status

    async def _check_api(self) -> Dict:
        """Check if API is responsive"""
        try:
            # Simple check - if we can execute this, API is running
            return {"status": "pass", "message": "API responsive"}
        except Exception as e:
            return {"status": "fail", "message": str(e)}

    async def _check_memory(self) -> Dict:
        """Check memory usage"""
        try:
            import psutil
            memory = psutil.virtual_memory()
            memory_usage_percent = memory.percent

            if memory_usage_percent > 90:
                return {"status": "warn", "usage_percent": memory_usage_percent}
            else:
                return {"status": "pass", "usage_percent": memory_usage_percent}
        except ImportError:
            return {"status": "skip", "message": "psutil not available"}
        except Exception as e:
            return {"status": "fail", "message": str(e)}

    async def _check_disk(self) -> Dict:
        """Check disk usage"""
        try:
            import psutil
            disk = psutil.disk_usage('/')
            disk_usage_percent = disk.percent

            if disk_usage_percent > 90:
                return {"status": "warn", "usage_percent": disk_usage_percent}
            else:
                return {"status": "pass", "usage_percent": disk_usage_percent}
        except ImportError:
            return {"status": "skip", "message": "psutil not available"}
        except Exception as e:
            return {"status": "fail", "message": str(e)}
