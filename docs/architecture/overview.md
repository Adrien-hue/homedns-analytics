# HomeDNS Analytics Architecture Overview

> **Current architecture version:** Sprint 02 / `v0.2.0`  
> **Primary runtime target:** Raspberry Pi 3B, Debian 13 ARM64  
> **Current production component:** Go DNS forwarder

---

## 1. Purpose

HomeDNS Analytics is a lightweight, self-hosted DNS filtering and network analytics platform designed to run continuously on a Raspberry Pi.

The long-term platform will provide DNS forwarding, filtering, caching, query analytics, a management API, and a web dashboard. The current `v0.2.0` architecture intentionally implements only the first production component: a reliable and measurable DNS forwarder.

The current system can:

- receive DNS queries over UDP and TCP;
- forward queries to one configured upstream resolver;
- return upstream responses to the requesting client;
- expose a local health endpoint;
- maintain in-memory runtime counters;
- report build and runtime metadata;
- run repeatable performance benchmarks;
- collect benchmark-time process and system resource metrics;
- be packaged, deployed, validated, and rolled back through versioned releases.

The following capabilities remain outside the current release:

- domain filtering;
- blacklists and whitelists;
- response caching;
- query-history persistence;
- per-client analytics;
- SQLite integration;
- the FastAPI management service;
- the React dashboard;
- multiple upstream resolvers and failover;
- encrypted DNS protocols.

---

## 2. Current System Context

```text
+--------------------+
| Network client     |
| Laptop, phone, IoT |
+---------+----------+
          |
          | DNS over UDP or TCP
          v
+---------------------------+
| HomeDNS DNS Forwarder     |
| Go service on Raspberry Pi|
|                           |
| - UDP listener            |
| - TCP listener            |
| - Request handler         |
| - Upstream forwarder      |
| - Runtime metrics         |
| - Health endpoint         |
+-------------+-------------+
              |
              | DNS over UDP or TCP
              v
+---------------------------+
| Configured upstream DNS   |
| Example: 1.1.1.1:53       |
+---------------------------+
```

The DNS service is the only application component that must remain continuously available in Sprint 02.

Development, release packaging, and most validation commands are executed from the developer workstation. The Raspberry Pi runs the released ARM64 binary under `systemd`.

---

## 3. Main Components

### 3.1 DNS service

The Go DNS service owns the runtime DNS request path.

Responsibilities:

- load and validate configuration;
- start UDP and TCP DNS listeners;
- receive DNS messages through `github.com/miekg/dns`;
- forward requests to the configured upstream resolver;
- translate upstream failures into safe DNS responses;
- record in-memory counters and latency measurements;
- expose readiness and health state;
- stop cleanly when the process receives a shutdown signal.

Detailed design: [`dns-forwarder.md`](dns-forwarder.md)

### 3.2 Health service

The health service runs inside the same process as the DNS service and listens on a loopback address.

It reports:

- overall status and readiness;
- binary version, commit, and build time;
- process uptime;
- UDP and TCP listener state;
- configured DNS and upstream addresses;
- request, response, timeout, and error counters;
- average upstream latency.

The health endpoint is also used by the release installer to validate a deployment before considering it successful.

### 3.3 Benchmark system

The same `homedns-dns` binary exposes a `benchmark` subcommand.

The benchmark system measures:

- direct queries to the configured upstream;
- forwarded queries through HomeDNS;
- UDP and TCP behavior;
- sequential and concurrent workloads;
- latency, throughput, failure, and timeout rates;
- benchmark-process resource usage;
- DNS-service resource usage;
- Raspberry Pi load, memory, temperature, and throttling state.

Detailed design: [`benchmark-system.md`](benchmark-system.md)

### 3.4 Deployment system

Release scripts on the developer workstation:

1. cross-compile the Go binary for Linux ARM64;
2. build a versioned release archive;
3. transfer the archive and installer over SSH;
4. validate and extract the release on the Raspberry Pi;
5. atomically activate the new release;
6. restart the service;
7. verify readiness;
8. automatically restore the previous release when validation fails.

Detailed design: [`deployment.md`](deployment.md)

### 3.5 Runtime resource monitoring

Resource collection is active during benchmark execution. It collects before, periodic, and after measurements from Linux interfaces such as `/proc`, `/sys`, and `vcgencmd`.

Detailed design: [`runtime-monitoring.md`](runtime-monitoring.md)

---

## 4. Runtime Topology

```text
Developer workstation
macOS
|
| Git, Go tests, release packaging, SSH deployment
|
+---------------------------------------------------+
                                                    |
                                                    v
                                      Raspberry Pi 3B
                                      Debian 13 ARM64
                                      |
                                      +-- systemd
                                      |    |
                                      |    +-- homedns-dns.service
                                      |         |
                                      |         +-- DNS UDP/TCP :53
                                      |         +-- Health HTTP 127.0.0.1:8081
                                      |
                                      +-- /opt/homedns
                                           |
                                           +-- current -> releases/<release-id>
                                           +-- releases/
                                           +-- shared/config/
                                           +-- shared/data/
                                           +-- shared/logs/
                                           +-- shared/benchmarks/
```

The release directory is immutable after installation. Persistent configuration, data, logs, and benchmark history live under `shared/`.

---

## 5. Primary Runtime Flows

### 5.1 DNS request flow

```text
Client
  |
  | DNS query
  v
UDP or TCP listener
  |
  v
Request handler
  |
  v
Configured upstream forwarder
  |
  | DNS response or error
  v
Request handler
  |
  v
Client response
```

The current forwarder does not cache or filter the request. Every accepted query is sent to the configured upstream.

### 5.2 Health flow

```text
Operator or deployment script
              |
              | HTTP GET /health
              v
Health handler
              |
              +-- listener readiness
              +-- version metadata
              +-- upstream configuration
              +-- runtime metrics snapshot
              |
              v
JSON health response
```

### 5.3 Benchmark flow

```text
Benchmark CLI
     |
     v
Benchmark service
     |
     v
Report runner
     |
     +-- resource collector starts
     +-- benchmark suite runs
     +-- resource collector stops
     +-- results are classified
     +-- canonical JSON report is written
```

---

## 6. Architectural Principles

### 6.1 DNS availability before analytics

The DNS request path must remain independent from future dashboard and analytics components. A management API or dashboard failure must not interrupt DNS resolution.

### 6.2 One implementation per responsibility

The codebase avoids duplicate configuration models, alternate forwarding paths, parallel metric registries, and competing lifecycle implementations.

### 6.3 Small current scope, explicit extension points

Sprint 02 does not implement future features prematurely. Package boundaries nevertheless leave room for:

- rule evaluation before forwarding;
- response caching;
- multiple upstream strategies;
- asynchronous query-event publication;
- persistent analytics;
- richer internal control endpoints.

### 6.4 Measurement before optimization

The benchmark system and Sprint 02 baseline are part of the architecture, not temporary tooling. Future changes must be compared against measured latency, reliability, throughput, and resource usage.

### 6.5 Raspberry Pi constraints are first-class

The architecture prioritizes:

- low steady-state memory use;
- low CPU use;
- no permanent Node.js process;
- simple systemd supervision;
- no Docker requirement for the first production version;
- safe operation on a 1 GB Raspberry Pi 3B.

---

## 7. Current Limitations

The `v0.2.0` architecture has deliberate limitations:

- one configured upstream resolver;
- no upstream retry or failover;
- one service process;
- no response cache;
- no filtering engine;
- no persistent query events;
- no client identification;
- no management API;
- no dashboard;
- benchmark service-readiness metadata is not yet captured before and after runs;
- heavy concurrent TCP benchmarks can degrade because both direct and forwarded paths experience timeouts under sustained load.

These limitations are documented constraints, not hidden behavior.

---

## 8. Architecture Documentation Map

- [`dns-forwarder.md`](dns-forwarder.md) — DNS runtime and request handling.
- [`benchmark-system.md`](benchmark-system.md) — benchmark execution, profiles, scenarios, and reports.
- [`deployment.md`](deployment.md) — build, release, activation, validation, and rollback.
- [`runtime-monitoring.md`](runtime-monitoring.md) — process and Raspberry Pi resource collection.

Operational commands belong in `docs/operations/`. Sprint history and delivery status belong in `docs/sprints/`.
