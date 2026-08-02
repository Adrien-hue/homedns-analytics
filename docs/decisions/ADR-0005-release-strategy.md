# ADR-0005 --- Atomic Release Strategy

## Status

Accepted

## Context

Deployments must be safe on a remote Raspberry Pi.

## Decision

Deploy immutable release directories, switch a `current` symlink
atomically and rollback automatically if validation fails.

## Consequences

-   Safe deployments
-   Fast rollback
-   Reproducible releases
