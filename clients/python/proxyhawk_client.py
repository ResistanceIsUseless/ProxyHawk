#!/usr/bin/env python3
"""
ProxyHawk WebSocket Client Library
==================================

Python client library for interacting with ProxyHawk's WebSocket API.
Provides both synchronous and asynchronous geographic DNS testing capabilities.

Example usage:
    # Basic usage
    client = ProxyHawkClient("ws://localhost:8888/ws")
    await client.connect()
    result = await client.test_domain("example.com")
    print(f"Geographic differences: {result['has_geographic_differences']}")
    
    # Batch testing
    results = await client.batch_test(["example.com", "google.com", "github.com"])
    for result in results:
        print(f"{result['domain']}: {len(result['region_results'])} regions tested")
    
    # Subscribe to domain updates
    await client.subscribe("example.com")
    client.on_domain_update(lambda domain, data: print(f"Update for {domain}: {data}"))
"""

import json
import logging
import time
import asyncio
import uuid
from typing import Dict, List, Optional, Callable, Any
from dataclasses import dataclass, asdict
from enum import Enum

try:
    import websockets
    from websockets.exceptions import ConnectionClosed, WebSocketException
except ImportError:
    raise ImportError("websockets library is required. Install with: pip install websockets")


class TestMode(Enum):
    """Testing modes supported by ProxyHawk."""
    BASIC = "basic"
    DETAILED = "detailed"
    COMPREHENSIVE = "comprehensive"


@dataclass
class ProxyHawkConfig:
    """Configuration for ProxyHawk client."""
    url: str = "ws://localhost:8888/ws"
    regions: List[str] = None
    test_mode: TestMode = TestMode.BASIC
    timeout: float = 30.0
    reconnect_delay: float = 5.0
    max_retries: int = 3
    
    def __post_init__(self):
        if self.regions is None:
            self.regions = ["us-west", "us-east", "eu-west"]


@dataclass
class DNSResult:
    """DNS lookup result from ProxyHawk."""
    query_time: str
    ip: str
    ttl: int
    record_type: str


@dataclass
class HTTPResult:
    """HTTP test result from ProxyHawk."""
    request_time: str
    status_code: int
    response_time_ms: int
    headers: Dict[str, str]
    server_header: str
    content_hash: str
    content_size: int
    remote_addr: str


@dataclass
class RegionResult:
    """Test results for a specific region."""
    region: str
    proxy_used: str
    dns_results: List[DNSResult]
    http_results: List[HTTPResult]
    response_time_ms: int
    success: bool
    error: str = ""


@dataclass
class TestSummary:
    """Summary of ProxyHawk test results."""
    unique_ips: List[str]
    unique_servers: List[str]
    response_time_diff_ms: int
    content_variations: Dict[str, int]
    geographic_spread: bool


@dataclass
class GeoTestResult:
    """Complete geographic test result."""
    domain: str
    tested_at: str
    has_geographic_differences: bool
    is_round_robin: bool
    confidence: float
    region_results: Dict[str, RegionResult]
    summary: TestSummary


class ProxyHawkClient:
    """
    WebSocket client for ProxyHawk geographic testing service.
    
    Features:
    - Async/sync domain testing
    - Batch testing capabilities
    - Domain subscription system
    - Auto-reconnection
    - Event-driven updates
    """
    
    def __init__(self, config: Optional[ProxyHawkConfig] = None):
        """Initialize ProxyHawk client."""
        self.config = config or ProxyHawkConfig()
        self.ws = None
        self.connected = False
        self.logger = logging.getLogger(__name__)
        
        # Message handling
        self._message_handlers: Dict[str, asyncio.Future] = {}
        self._subscriptions: Dict[str, List[Callable]] = {}
        self._message_counter = 0
        
        # Connection management
        self._reconnect_task = None
        self._heartbeat_task = None
        self._stop_event = asyncio.Event()
        
        # Event handlers
        self.on_connected: Optional[Callable] = None
        self.on_disconnected: Optional[Callable] = None
        self.on_error: Optional[Callable] = None
    
    async def connect(self) -> bool:
        """Connect to ProxyHawk WebSocket API."""
        try:
            self.logger.info(f"Connecting to ProxyHawk at {self.config.url}")
            self.ws = await websockets.connect(self.config.url)
            self.connected = True
            
            # Start message receiver
            asyncio.create_task(self._message_receiver())
            
            # Send initial configuration
            await self._send_config()
            
            # Start heartbeat
            self._heartbeat_task = asyncio.create_task(self._heartbeat())
            
            if self.on_connected:
                self.on_connected()
                
            self.logger.info("Connected to ProxyHawk successfully")
            return True
            
        except Exception as e:
            self.logger.error(f"Failed to connect to ProxyHawk: {e}")
            self.connected = False
            if self.on_error:
                self.on_error(e)
            return False
    
    async def disconnect(self):
        """Disconnect from ProxyHawk WebSocket API."""
        self.logger.info("Disconnecting from ProxyHawk")
        self.connected = False
        self._stop_event.set()
        
        # Cancel tasks
        if self._heartbeat_task:
            self._heartbeat_task.cancel()
        if self._reconnect_task:
            self._reconnect_task.cancel()
            
        # Close WebSocket
        if self.ws and not self.ws.closed:
            await self.ws.close()
            
        if self.on_disconnected:
            self.on_disconnected()
    
    async def test_domain(self, domain: str, regions: Optional[List[str]] = None) -> GeoTestResult:
        """
        Test a domain for geographic differences.
        
        Args:
            domain: Domain to test
            regions: Optional list of regions to test from
            
        Returns:
            GeoTestResult with complete test results
        """
        if not self.connected:
            raise RuntimeError("Not connected to ProxyHawk")
            
        message_id = self._generate_message_id()
        test_regions = regions or self.config.regions
        
        request = {
            "type": "test",
            "id": message_id,
            "domain": domain,
            "data": {
                "domain": domain,
                "regions": test_regions,
                "mode": self.config.test_mode.value
            },
            "timestamp": time.time()
        }
        
        # Set up response handler
        future = asyncio.Future()
        self._message_handlers[message_id] = future
        
        try:
            # Send request
            await self.ws.send(json.dumps(request))
            self.logger.debug(f"Sent test request for domain: {domain}")
            
            # Wait for response
            response = await asyncio.wait_for(future, timeout=self.config.timeout)
            
            if response.get("error"):
                raise RuntimeError(f"ProxyHawk error: {response['error']}")
                
            # Parse result
            result_data = json.loads(response["data"]) if isinstance(response["data"], str) else response["data"]
            return self._parse_geo_test_result(result_data)
            
        except asyncio.TimeoutError:
            raise TimeoutError(f"Test timeout for domain: {domain}")
        finally:
            self._message_handlers.pop(message_id, None)
    
    async def batch_test(self, domains: List[str], regions: Optional[List[str]] = None) -> List[GeoTestResult]:
        """
        Test multiple domains efficiently.
        
        Args:
            domains: List of domains to test
            regions: Optional list of regions to test from
            
        Returns:
            List of GeoTestResult objects
        """
        if not self.connected:
            raise RuntimeError("Not connected to ProxyHawk")
            
        if not domains:
            return []
            
        message_id = self._generate_message_id()
        test_regions = regions or self.config.regions
        
        request = {
            "type": "batch_test",
            "id": message_id,
            "data": {
                "domains": domains,
                "regions": test_regions,
                "mode": self.config.test_mode.value
            },
            "timestamp": time.time()
        }
        
        # Set up response handler
        future = asyncio.Future()
        results = []
        self._message_handlers[message_id] = future
        
        try:
            # Send request
            await self.ws.send(json.dumps(request))
            self.logger.info(f"Sent batch test request for {len(domains)} domains")
            
            # Collect results (may come in multiple messages)
            while len(results) < len(domains):
                response = await asyncio.wait_for(future, timeout=self.config.timeout)
                
                if response.get("error"):
                    self.logger.error(f"Batch test error: {response['error']}")
                    break
                    
                if response["type"] == "batch_result":
                    # Final batch results
                    batch_data = json.loads(response["data"]) if isinstance(response["data"], str) else response["data"]
                    for result_data in batch_data:
                        results.append(self._parse_geo_test_result(result_data))
                    break
                    
                elif response["type"] == "batch_partial":
                    # Partial results
                    partial_data = json.loads(response["data"]) if isinstance(response["data"], str) else response["data"]
                    for result_data in partial_data.get("results", []):
                        results.append(self._parse_geo_test_result(result_data))
                    
                    # Reset future for next partial result
                    if len(results) < len(domains):
                        future = asyncio.Future()
                        self._message_handlers[message_id] = future
            
            return results
            
        except asyncio.TimeoutError:
            raise TimeoutError(f"Batch test timeout after receiving {len(results)}/{len(domains)} results")
        finally:
            self._message_handlers.pop(message_id, None)
    
    async def subscribe(self, domain: str, callback: Optional[Callable] = None):
        """
        Subscribe to updates for a domain.
        
        Args:
            domain: Domain to subscribe to
            callback: Optional callback for domain updates
        """
        if not self.connected:
            raise RuntimeError("Not connected to ProxyHawk")
            
        message_id = self._generate_message_id()
        
        request = {
            "type": "subscribe",
            "id": message_id,
            "domain": domain,
            "timestamp": time.time()
        }
        
        # Add callback to subscriptions
        if callback:
            if domain not in self._subscriptions:
                self._subscriptions[domain] = []
            self._subscriptions[domain].append(callback)
        
        # Set up response handler
        future = asyncio.Future()
        self._message_handlers[message_id] = future
        
        try:
            await self.ws.send(json.dumps(request))
            response = await asyncio.wait_for(future, timeout=10.0)  # Short timeout for subscription
            
            if response.get("error"):
                raise RuntimeError(f"Subscription error: {response['error']}")
                
            self.logger.info(f"Subscribed to updates for domain: {domain}")
            
        finally:
            self._message_handlers.pop(message_id, None)
    
    async def unsubscribe(self, domain: str):
        """Unsubscribe from domain updates."""
        if not self.connected:
            return
            
        message_id = self._generate_message_id()
        
        request = {
            "type": "unsubscribe",
            "id": message_id,
            "domain": domain,
            "timestamp": time.time()
        }
        
        future = asyncio.Future()
        self._message_handlers[message_id] = future
        
        try:
            await self.ws.send(json.dumps(request))
            await asyncio.wait_for(future, timeout=10.0)
            
            # Remove local subscriptions
            self._subscriptions.pop(domain, None)
            
            self.logger.info(f"Unsubscribed from domain: {domain}")
            
        except asyncio.TimeoutError:
            self.logger.warning(f"Unsubscribe timeout for domain: {domain}")
        finally:
            self._message_handlers.pop(message_id, None)
    
    def on_domain_update(self, callback: Callable[[str, Dict], None]):
        """Set global domain update callback."""
        self._global_update_callback = callback
    
    async def get_regions(self) -> List[str]:
        """Get available regions from ProxyHawk."""
        if not self.connected:
            raise RuntimeError("Not connected to ProxyHawk")
            
        message_id = self._generate_message_id()
        
        request = {
            "type": "get_regions",
            "id": message_id,
            "timestamp": time.time()
        }
        
        future = asyncio.Future()
        self._message_handlers[message_id] = future
        
        try:
            await self.ws.send(json.dumps(request))
            response = await asyncio.wait_for(future, timeout=10.0)
            
            regions_data = json.loads(response["data"]) if isinstance(response["data"], str) else response["data"]
            return regions_data
            
        finally:
            self._message_handlers.pop(message_id, None)
    
    # Private methods
    
    async def _message_receiver(self):
        """Receive and route messages from ProxyHawk."""
        try:
            async for message in self.ws:
                try:
                    data = json.loads(message)
                    await self._handle_message(data)
                except json.JSONDecodeError as e:
                    self.logger.error(f"Failed to decode message: {e}")
                except Exception as e:
                    self.logger.error(f"Error handling message: {e}")
                    
        except ConnectionClosed:
            self.logger.info("WebSocket connection closed")
            self.connected = False
        except Exception as e:
            self.logger.error(f"Message receiver error: {e}")
            self.connected = False
            if self.on_error:
                self.on_error(e)
    
    async def _handle_message(self, message: Dict):
        """Handle incoming message from ProxyHawk."""
        message_type = message.get("type", "")
        message_id = message.get("id", "")
        
        # Route to specific handlers
        if message_id and message_id in self._message_handlers:
            future = self._message_handlers[message_id]
            if not future.done():
                future.set_result(message)
        
        # Handle special message types
        elif message_type == "welcome":
            self.logger.info("Received welcome from ProxyHawk")
            
        elif message_type == "domain_update":
            domain = message.get("domain", "")
            self._handle_domain_update(domain, message.get("data", {}))
            
        elif message_type == "error":
            self.logger.error(f"ProxyHawk error: {message.get('error', 'Unknown error')}")
            
        else:
            self.logger.debug(f"Unhandled message type: {message_type}")
    
    def _handle_domain_update(self, domain: str, data: Dict):
        """Handle domain update notification."""
        # Call domain-specific callbacks
        if domain in self._subscriptions:
            for callback in self._subscriptions[domain]:
                try:
                    callback(domain, data)
                except Exception as e:
                    self.logger.error(f"Error in domain callback for {domain}: {e}")
        
        # Call global callback if set
        if hasattr(self, '_global_update_callback') and self._global_update_callback:
            try:
                self._global_update_callback(domain, data)
            except Exception as e:
                self.logger.error(f"Error in global update callback: {e}")
    
    async def _send_config(self):
        """Send initial client configuration to ProxyHawk."""
        request = {
            "type": "set_config",
            "data": {
                "regions": self.config.regions,
                "test_mode": self.config.test_mode.value
            },
            "timestamp": time.time()
        }
        
        await self.ws.send(json.dumps(request))
    
    async def _heartbeat(self):
        """Send periodic ping messages to maintain connection."""
        while self.connected and not self._stop_event.is_set():
            try:
                await asyncio.sleep(30)  # Ping every 30 seconds
                if self.connected:
                    ping_msg = {
                        "type": "ping",
                        "id": self._generate_message_id(),
                        "timestamp": time.time()
                    }
                    await self.ws.send(json.dumps(ping_msg))
                    
            except Exception as e:
                self.logger.warning(f"Heartbeat error: {e}")
                break
    
    def _generate_message_id(self) -> str:
        """Generate unique message ID."""
        self._message_counter += 1
        return f"python_client_{int(time.time() * 1000)}_{self._message_counter}"
    
    def _parse_geo_test_result(self, data: Dict) -> GeoTestResult:
        """Parse raw test result data into GeoTestResult object."""
        # Parse region results
        region_results = {}
        for region_name, region_data in data.get("region_results", {}).items():
            dns_results = [DNSResult(**dns) for dns in region_data.get("dns_results", [])]
            http_results = [HTTPResult(**http) for http in region_data.get("http_results", [])]
            
            region_results[region_name] = RegionResult(
                region=region_data["region"],
                proxy_used=region_data["proxy_used"],
                dns_results=dns_results,
                http_results=http_results,
                response_time_ms=region_data["response_time"],
                success=region_data["success"],
                error=region_data.get("error", "")
            )
        
        # Parse summary
        summary_data = data.get("summary", {})
        summary = TestSummary(
            unique_ips=summary_data.get("unique_ips", []),
            unique_servers=summary_data.get("unique_servers", []),
            response_time_diff_ms=summary_data.get("response_time_diff", 0),
            content_variations=summary_data.get("content_variations", {}),
            geographic_spread=summary_data.get("geographic_spread", False)
        )
        
        return GeoTestResult(
            domain=data["domain"],
            tested_at=data["tested_at"],
            has_geographic_differences=data["has_geographic_differences"],
            is_round_robin=data["is_round_robin"],
            confidence=data["confidence"],
            region_results=region_results,
            summary=summary
        )


# Synchronous wrapper for convenience
class SyncProxyHawkClient:
    """
    Synchronous wrapper for ProxyHawkClient.
    
    Provides a simpler interface for non-async applications.
    """
    
    def __init__(self, config: Optional[ProxyHawkConfig] = None):
        self.client = ProxyHawkClient(config)
        self.loop = None
    
    def connect(self) -> bool:
        """Connect to ProxyHawk."""
        return self._run_async(self.client.connect())
    
    def disconnect(self):
        """Disconnect from ProxyHawk."""
        self._run_async(self.client.disconnect())
    
    def test_domain(self, domain: str, regions: Optional[List[str]] = None) -> GeoTestResult:
        """Test a domain synchronously."""
        return self._run_async(self.client.test_domain(domain, regions))
    
    def batch_test(self, domains: List[str], regions: Optional[List[str]] = None) -> List[GeoTestResult]:
        """Test multiple domains synchronously."""
        return self._run_async(self.client.batch_test(domains, regions))
    
    def get_regions(self) -> List[str]:
        """Get available regions synchronously."""
        return self._run_async(self.client.get_regions())
    
    def _run_async(self, coro):
        """Run async coroutine in sync context."""
        if self.loop is None or self.loop.is_closed():
            self.loop = asyncio.new_event_loop()
            asyncio.set_event_loop(self.loop)
        
        return self.loop.run_until_complete(coro)


# Example usage and testing
async def main():
    """Example usage of ProxyHawk client."""
    logging.basicConfig(level=logging.INFO)
    
    # Configure client
    config = ProxyHawkConfig(
        url="ws://localhost:8888/ws",
        regions=["us-west", "us-east", "eu-west"],
        test_mode=TestMode.BASIC,
        timeout=30.0
    )
    
    client = ProxyHawkClient(config)
    
    try:
        # Connect
        if not await client.connect():
            print("Failed to connect to ProxyHawk")
            return
        
        # Test single domain
        print("Testing single domain...")
        result = await client.test_domain("example.com")
        print(f"Domain: {result.domain}")
        print(f"Geographic differences: {result.has_geographic_differences}")
        print(f"Round-robin detected: {result.is_round_robin}")
        print(f"Confidence: {result.confidence:.2f}")
        print(f"Regions tested: {len(result.region_results)}")
        print()
        
        # Test multiple domains
        print("Testing multiple domains...")
        domains = ["google.com", "github.com", "stackoverflow.com"]
        results = await client.batch_test(domains)
        
        for result in results:
            print(f"{result.domain}: "
                  f"{'🌍' if result.has_geographic_differences else '📍'} "
                  f"{'🔄' if result.is_round_robin else '🎯'} "
                  f"({len(result.region_results)} regions)")
        
        # Get available regions
        print(f"\nAvailable regions: {await client.get_regions()}")
        
    except Exception as e:
        print(f"Error: {e}")
    
    finally:
        await client.disconnect()


if __name__ == "__main__":
    # For direct execution
    asyncio.run(main())