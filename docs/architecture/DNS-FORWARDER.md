# DNS Forwarder Architecture

## Purpose
The DNS Forwarder is the core service of HomeDNS. It receives DNS requests from LAN clients, optionally processes them, forwards them to an upstream resolver, and returns the response.

## Design Principles
- Keep the DNS request path as short as possible.
- Never block on database operations.
- Keep extension points for cache, filtering and metrics.
- Prefer composition over monolithic code.

## Package Layout
- cmd/server: application entrypoint
- internal/config: YAML configuration
- internal/dnsserver: UDP/TCP listeners and handlers
- internal/upstream: upstream DNS client
- internal/health: HTTP health endpoint
- internal/logging: structured logging

## Future Extension Points
1. Filter middleware
2. Cache middleware
3. Metrics publisher
4. Event queue

## Error Handling
- Invalid packet → FORMERR
- Upstream timeout → SERVFAIL
- Graceful shutdown on SIGTERM/SIGINT

## Logging
JSON structured logs with request id, client IP, latency, upstream status.
