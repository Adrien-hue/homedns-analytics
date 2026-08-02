# ADR-0006 --- Runtime Resource Monitoring

## Status

Accepted

## Context

Benchmark results alone are insufficient to evaluate Raspberry Pi
health.

## Decision

Collect CPU, memory, temperature, thread count and throttling metrics
during benchmark execution.

## Consequences

-   Performance correlated with hardware behavior
-   Easier bottleneck analysis
-   Better long-term comparisons
