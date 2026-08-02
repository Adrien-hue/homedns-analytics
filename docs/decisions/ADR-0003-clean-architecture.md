# ADR-0003 --- Clean Architecture

## Status

Accepted

## Context

The project will grow beyond a simple DNS forwarder.

## Decision

Separate application, benchmark, DNS server, health, metrics and
infrastructure concerns into independent packages.

## Consequences

-   Better testability
-   Reduced coupling
-   Easier future extensions
