#!/usr/bin/env python3
"""
Load testing suite for Claude Agent SDK integration
Tests capacity for 10,000+ concurrent users
"""
import argparse
import asyncio
import json
import random
import time
from collections import defaultdict
from datetime import datetime
from typing import List, Dict

import aiohttp
from dataclasses import dataclass
from statistics import mean, median, stdev


@dataclass
class TestResult:
    """Individual test result"""
    success: bool
    duration_ms: float
    status_code: int
    error: str = ""
    timestamp: datetime = None


class LoadTester:
    """Load testing framework for Claude agents"""

    def __init__(self, api_url: str, anthropic_key: str, verbose: bool = False):
        self.api_url = api_url.rstrip('/')
        self.anthropic_key = anthropic_key
        self.verbose = verbose
        self.results: List[TestResult] = []
        self.agent_ids: List[str] = []
        self.session_ids: Dict[str, str] = {}  # user_id -> session_id

    async def spawn_agent(self, session: aiohttp.ClientSession, user_id: str) -> TestResult:
        """Spawn a Claude agent for a user"""
        start = time.time()

        payload = {
            "user_id": user_id,
            "anthropic_api_key": self.anthropic_key,
            "cores": 4,
            "memory": 8192,
            "max_concurrent": 100,
        }

        try:
            async with session.post(
                f"{self.api_url}/v1/agents/spawn",
                json=payload,
                timeout=aiohttp.ClientTimeout(total=60)
            ) as response:
                duration = (time.time() - start) * 1000
                success = response.status == 200

                if success:
                    data = await response.json()
                    agent_id = data.get("agent_id")
                    self.agent_ids.append(agent_id)

                    if self.verbose:
                        print(f"✓ Spawned agent {agent_id} for {user_id}")

                return TestResult(
                    success=success,
                    duration_ms=duration,
                    status_code=response.status,
                    timestamp=datetime.now()
                )

        except Exception as e:
            duration = (time.time() - start) * 1000
            if self.verbose:
                print(f"✗ Spawn failed for {user_id}: {e}")

            return TestResult(
                success=False,
                duration_ms=duration,
                status_code=0,
                error=str(e),
                timestamp=datetime.now()
            )

    async def chat_request(self, session: aiohttp.ClientSession, agent_url: str, user_id: str) -> TestResult:
        """Send a chat request to an agent"""
        start = time.time()

        prompts = [
            "Explain quantum computing in simple terms",
            "Write a Python function to calculate Fibonacci numbers",
            "What are the benefits of microservices architecture?",
            "How does machine learning differ from traditional programming?",
            "Describe the SOLID principles in software engineering",
        ]

        payload = {
            "prompt": random.choice(prompts),
            "user_id": user_id,
            "max_tokens": 500,
            "stream": False
        }

        try:
            async with session.post(
                f"https://{agent_url}/v1/agent/chat",
                json=payload,
                timeout=aiohttp.ClientTimeout(total=60)
            ) as response:
                duration = (time.time() - start) * 1000
                success = response.status == 200

                if success:
                    data = await response.json()
                    self.session_ids[user_id] = data.get("session_id", "")

                    if self.verbose:
                        tokens = data.get("usage", {}).get("total_tokens", 0)
                        print(f"✓ Chat request for {user_id} completed ({tokens} tokens, {duration:.0f}ms)")

                return TestResult(
                    success=success,
                    duration_ms=duration,
                    status_code=response.status,
                    timestamp=datetime.now()
                )

        except Exception as e:
            duration = (time.time() - start) * 1000
            if self.verbose:
                print(f"✗ Chat request failed for {user_id}: {e}")

            return TestResult(
                success=False,
                duration_ms=duration,
                status_code=0,
                error=str(e),
                timestamp=datetime.now()
            )

    async def cleanup_agent(self, session: aiohttp.ClientSession, agent_id: str) -> None:
        """Delete an agent"""
        try:
            async with session.delete(
                f"{self.api_url}/v1/agents/delete?agent_id={agent_id}",
                timeout=aiohttp.ClientTimeout(total=30)
            ) as response:
                if response.status == 200:
                    if self.verbose:
                        print(f"✓ Deleted agent {agent_id}")
        except Exception as e:
            if self.verbose:
                print(f"✗ Failed to delete agent {agent_id}: {e}")

    async def run_user_simulation(self, session: aiohttp.ClientSession, user_id: str, num_requests: int) -> List[TestResult]:
        """Simulate a single user making multiple requests"""
        results = []

        # Spawn agent
        spawn_result = await self.spawn_agent(session, user_id)
        results.append(spawn_result)

        if not spawn_result.success:
            return results

        # Get agent URL (assume last spawned agent)
        agent_url = f"{self.agent_ids[-1]}.deployments.vistara.dev"

        # Make chat requests
        for _ in range(num_requests):
            chat_result = await self.chat_request(session, agent_url, user_id)
            results.append(chat_result)

            # Small delay between requests
            await asyncio.sleep(random.uniform(0.1, 0.5))

        return results

    async def run_concurrent_load_test(
        self,
        num_users: int,
        requests_per_user: int,
        ramp_up_seconds: int
    ) -> List[TestResult]:
        """Run concurrent load test"""
        print(f"\n{'='*70}")
        print(f"Starting load test:")
        print(f"  - Users: {num_users}")
        print(f"  - Requests per user: {requests_per_user}")
        print(f"  - Total requests: {num_users * requests_per_user}")
        print(f"  - Ramp-up period: {ramp_up_seconds}s")
        print(f"{'='*70}\n")

        all_results = []
        connector = aiohttp.TCPConnector(limit=1000, limit_per_host=100)

        async with aiohttp.ClientSession(connector=connector) as session:
            tasks = []

            # Create tasks with ramp-up
            for i in range(num_users):
                user_id = f"load-test-user-{i:05d}"
                task = self.run_user_simulation(session, user_id, requests_per_user)
                tasks.append(task)

                # Ramp-up delay
                if ramp_up_seconds > 0 and i < num_users - 1:
                    await asyncio.sleep(ramp_up_seconds / num_users)

            # Wait for all tasks to complete
            print(f"\n⏳ Running {len(tasks)} user simulations...\n")
            results_list = await asyncio.gather(*tasks, return_exceptions=True)

            # Flatten results
            for result in results_list:
                if isinstance(result, list):
                    all_results.extend(result)
                elif isinstance(result, Exception):
                    print(f"✗ Task failed: {result}")

            # Cleanup agents
            print(f"\n🧹 Cleaning up {len(self.agent_ids)} agents...")
            cleanup_tasks = [self.cleanup_agent(session, aid) for aid in self.agent_ids]
            await asyncio.gather(*cleanup_tasks, return_exceptions=True)

        return all_results

    def analyze_results(self, results: List[TestResult]) -> Dict:
        """Analyze test results and generate report"""
        if not results:
            return {"error": "No results to analyze"}

        successful = [r for r in results if r.success]
        failed = [r for r in results if not r.success]

        durations = [r.duration_ms for r in successful]

        # Calculate percentiles
        def percentile(data, p):
            if not data:
                return 0
            sorted_data = sorted(data)
            index = int(len(sorted_data) * p)
            return sorted_data[min(index, len(sorted_data) - 1)]

        total_requests = len(results)
        success_rate = (len(successful) / total_requests * 100) if total_requests > 0 else 0

        # Calculate throughput
        if results:
            time_span = (results[-1].timestamp - results[0].timestamp).total_seconds()
            throughput = total_requests / time_span if time_span > 0 else 0
        else:
            throughput = 0

        # Error breakdown
        error_types = defaultdict(int)
        for r in failed:
            error_types[r.error[:50]] += 1  # Truncate error message

        return {
            "total_requests": total_requests,
            "successful": len(successful),
            "failed": len(failed),
            "success_rate": success_rate,
            "throughput_req_sec": throughput,
            "latency": {
                "min_ms": min(durations) if durations else 0,
                "max_ms": max(durations) if durations else 0,
                "mean_ms": mean(durations) if durations else 0,
                "median_ms": median(durations) if durations else 0,
                "stdev_ms": stdev(durations) if len(durations) > 1 else 0,
                "p50_ms": percentile(durations, 0.50),
                "p95_ms": percentile(durations, 0.95),
                "p99_ms": percentile(durations, 0.99),
            },
            "errors": dict(error_types)
        }

    def print_report(self, analysis: Dict) -> None:
        """Print formatted test report"""
        print(f"\n{'='*70}")
        print(f"LOAD TEST RESULTS")
        print(f"{'='*70}\n")

        print(f"📊 Summary:")
        print(f"  Total Requests:     {analysis['total_requests']:,}")
        print(f"  Successful:         {analysis['successful']:,}")
        print(f"  Failed:             {analysis['failed']:,}")
        print(f"  Success Rate:       {analysis['success_rate']:.2f}%")
        print(f"  Throughput:         {analysis['throughput_req_sec']:.2f} req/sec")

        print(f"\n⏱️  Latency (ms):")
        latency = analysis['latency']
        print(f"  Min:                {latency['min_ms']:.2f}")
        print(f"  Max:                {latency['max_ms']:.2f}")
        print(f"  Mean:               {latency['mean_ms']:.2f}")
        print(f"  Median (p50):       {latency['median_ms']:.2f}")
        print(f"  p95:                {latency['p95_ms']:.2f}")
        print(f"  p99:                {latency['p99_ms']:.2f}")
        print(f"  Std Dev:            {latency['stdev_ms']:.2f}")

        if analysis.get('errors'):
            print(f"\n❌ Errors:")
            for error, count in sorted(analysis['errors'].items(), key=lambda x: -x[1])[:5]:
                print(f"  [{count:3d}x] {error}")

        print(f"\n{'='*70}")

        # Pass/Fail criteria
        print(f"\n✅ PASS/FAIL Criteria:")
        checks = [
            ("Success Rate", analysis['success_rate'], ">=", 95.0, "%"),
            ("p95 Latency", latency['p95_ms'], "<", 5000, "ms"),
            ("p99 Latency", latency['p99_ms'], "<", 10000, "ms"),
            ("Throughput", analysis['throughput_req_sec'], ">", 10, "req/sec"),
        ]

        all_passed = True
        for name, actual, op, expected, unit in checks:
            if op == ">=":
                passed = actual >= expected
            elif op == "<":
                passed = actual < expected
            elif op == ">":
                passed = actual > expected
            else:
                passed = False

            status = "✓ PASS" if passed else "✗ FAIL"
            print(f"  {status}  {name}: {actual:.2f}{unit} ({op} {expected}{unit})")
            all_passed = all_passed and passed

        print(f"\n{'='*70}")
        if all_passed:
            print(f"🎉 ALL CHECKS PASSED - System ready for 10k users!")
        else:
            print(f"⚠️  SOME CHECKS FAILED - Review configuration and capacity")
        print(f"{'='*70}\n")


async def main():
    parser = argparse.ArgumentParser(description="Load test Claude Agent SDK integration")
    parser.add_argument("--api-url", required=True, help="Agent manager API URL")
    parser.add_argument("--anthropic-key", required=True, help="Anthropic API key")
    parser.add_argument("--users", type=int, default=100, help="Number of concurrent users")
    parser.add_argument("--requests", type=int, default=10, help="Requests per user")
    parser.add_argument("--ramp-up", type=int, default=10, help="Ramp-up period in seconds")
    parser.add_argument("--verbose", action="store_true", help="Verbose output")
    parser.add_argument("--output", help="Output JSON file for results")

    args = parser.parse_args()

    # Initialize tester
    tester = LoadTester(args.api_url, args.anthropic_key, args.verbose)

    # Run load test
    start_time = time.time()
    results = await tester.run_concurrent_load_test(
        num_users=args.users,
        requests_per_user=args.requests,
        ramp_up_seconds=args.ramp_up
    )
    total_time = time.time() - start_time

    print(f"\n⏱️  Total test duration: {total_time:.2f} seconds")

    # Analyze and print results
    analysis = tester.analyze_results(results)
    tester.print_report(analysis)

    # Save to file if requested
    if args.output:
        output_data = {
            "test_config": {
                "users": args.users,
                "requests_per_user": args.requests,
                "total_duration_seconds": total_time,
            },
            "results": analysis,
            "timestamp": datetime.now().isoformat()
        }

        with open(args.output, 'w') as f:
            json.dump(output_data, f, indent=2)

        print(f"\n💾 Results saved to: {args.output}")


if __name__ == "__main__":
    asyncio.run(main())
