# ADR-0002 --- Go for the DNS Engine

## Status

Accepted

## Context

The DNS forwarder requires low latency, concurrency and a small memory
footprint on Raspberry Pi.

## Decision

Implement the DNS service in Go.

## Consequences

-   Excellent runtime performance
-   Simple static binaries
-   Low resource usage
