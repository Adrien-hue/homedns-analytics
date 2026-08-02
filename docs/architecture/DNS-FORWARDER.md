# DNS Forwarder Architecture

> **Component:** `homedns-dns`  
> **Language:** Go  
> **DNS library:** `github.com/miekg/dns`  
> **Current release scope:** Forwarding MVP

---

## 1. Purpose

The DNS forwarder is the first production application component of HomeDNS Analytics.

It receives DNS queries from network clients over UDP or TCP, sends each query to one configured upstream DNS resolver, and returns the upstream response to the original client.

The Sprint 02 implementation deliberately excludes filtering, caching, persistence, and multi-upstream strategies. Its purpose is to establish a reliable request path and a measurable performance baseline.

---

## 2. Component View

```text
+------------------------------------------------+
| homedns-dns process                            |
|                                                |
|  +----------------+      +----------------+    |
|  | UDP DNS server |      | TCP DNS server |    |
|  +--------+-------+      +--------+-------+    |
|           |                       |            |
|           +-----------+-----------+            |
|                       v                        |
|              +------------------+              |
|              | DNS handler      |              |
|              |                  |              |
|              | - validation     |              |
|              | - metrics        |              |
|              | - error mapping  |              |
|              +--------+---------+              |
|                       |                        |
|                       v                        |
|              +------------------+              |
|              | Upstream client  |              |
|              | UDP or TCP       |              |
|              +--------+---------+              |
|                       |                        |
|  +--------------------+---------------------+  |
|  | Health server and runtime metric state   |  |
|  +------------------------------------------+  |
+------------------------------------------------+
                        |
                        v
             Configured upstream resolver
```

---

## 3. Package Responsibilities

The code is split into small packages with one primary responsibility each.

### `internal/config`

- configuration model;
- YAML loading;
- defaults;
- validation;
- derived DNS and health addresses.

### `internal/dnsserver`

- UDP and TCP listener lifecycle;
- DNS handler integration;
- listener readiness state;
- graceful shutdown.

### `internal/upstream`

- upstream DNS exchange;
- protocol selection;
- timeout enforcement;
- upstream error wrapping.

### `internal/metrics`

- in-memory counters;
- request and response classification;
- upstream latency aggregation;
- thread-safe snapshots.

### `internal/health`

- health-state model;
- JSON HTTP handler;
- loopback health server;
- readiness representation.

### `internal/app`

- dependency construction;
- startup order;
- rollback when startup partially fails;
- coordinated shutdown;
- health-state composition.

### `internal/version`

- release version;
- commit identifier;
- build timestamp;
- formatted version output.

---

## 4. Startup Sequence

```text
Load configuration
       |
       v
Validate configuration
       |
       v
Construct metrics and upstream client
       |
       v
Construct DNS servers and health server
       |
       v
Start UDP and TCP DNS listeners
       |
       v
Start health server
       |
       v
Report application ready
```

The service must not report readiness unless both DNS listeners are active.

If the health server fails after the DNS listeners start, the application stops the DNS servers before returning the startup error. This prevents a partially initialized process from continuing to serve traffic without operational visibility.

---

## 5. DNS Request Lifecycle

```text
1. Receive DNS message
2. Record request protocol
3. Forward the message upstream using the same transport
4. Record upstream latency
5. Classify the upstream response or error
6. Write a DNS response to the client
7. Record response-write failures if present
```

### 5.1 UDP queries

UDP requests are received by the UDP listener and forwarded upstream over UDP.

This is the normal path for most DNS traffic.

### 5.2 TCP queries

TCP requests are received by the TCP listener and forwarded upstream over TCP.

TCP support is required for valid DNS behavior, including large responses and clients that intentionally select TCP.

### 5.3 Transport preservation

The current implementation keeps the request transport consistent between the client-facing listener and the upstream exchange:

```text
Client UDP -> HomeDNS UDP -> Upstream UDP
Client TCP -> HomeDNS TCP -> Upstream TCP
```

Future versions may introduce transport policies, connection reuse, or encrypted upstream protocols. These are not part of Sprint 02.

---

## 6. Error Handling

The forwarder must always return a valid DNS-level outcome where possible.

Tracked failure categories include:

- upstream timeout;
- upstream exchange error;
- invalid request;
- response write failure;
- recovered handler panic.

When the upstream exchange fails, the handler returns a server-failure response rather than leaving the client without a DNS message.

Errors are also written through structured logs with fields such as:

- network protocol;
- upstream address;
- elapsed latency;
- timeout classification;
- wrapped error message.

---

## 7. Runtime Metrics

The service maintains in-memory counters for the current process lifetime.

Current metrics include:

- total requests;
- UDP requests;
- TCP requests;
- successful `NOERROR` responses;
- `NXDOMAIN` responses;
- `SERVFAIL` responses;
- other response codes;
- upstream timeouts;
- upstream errors;
- invalid requests;
- response-write errors;
- recovered handler panics;
- average upstream latency.

These counters are exposed through the health endpoint.

They are not persisted across service restarts in Sprint 02.

---

## 8. Health and Readiness

The health server listens on the configured loopback address, currently:

```text
127.0.0.1:8081
```

The endpoint reports:

- `status`;
- `ready`;
- release version;
- commit;
- build time;
- uptime;
- DNS listener state;
- configured DNS address;
- configured upstream address;
- runtime metrics.

The endpoint is intended for:

- local operational checks;
- deployment validation;
- future internal supervision and monitoring.

It is not currently exposed as a public management API.

---

## 9. Shutdown Sequence

```text
Shutdown signal
      |
      v
Stop advertising health readiness
      |
      v
Shut down health server
      |
      v
Shut down UDP and TCP DNS servers
      |
      v
Return combined shutdown result
```

Graceful shutdown prevents abandoned listeners and gives in-flight handlers an opportunity to finish within the configured shutdown context.

`systemd` supervises the process and starts it automatically after boot.

---

## 10. Configuration Boundary

The current service configuration controls:

- DNS listen host and port;
- health listen host and port;
- upstream resolver address;
- upstream timeout;
- logging behavior where configured.

The service does not dynamically reload configuration. Changes require a release or service restart.

---

## 11. Testing Strategy

The forwarder is validated at multiple levels:

### Unit tests

- configuration validation;
- metrics behavior;
- health serialization;
- upstream error classification;
- version output.

### Controlled integration tests

A local test DNS server provides deterministic upstream behavior for:

- successful UDP exchanges;
- successful TCP exchanges;
- delayed responses;
- timeouts;
- malformed or failed exchanges;
- listener lifecycle.

### Raspberry Pi validation

The released ARM64 binary is validated through:

- `systemd` status;
- health endpoint;
- real `dig` queries;
- benchmark profiles;
- resource collection;
- deployment restart and rollback behavior.

---

## 12. Current Performance Characteristics

The Sprint 02 baseline confirms:

- the quick workload completes without failures;
- the service remains stable under validation and endurance workloads;
- CPU and memory use remain low on the Raspberry Pi 3B;
- no thermal throttling occurs;
- sustained concurrent TCP workloads can experience latency growth and timeouts;
- the direct upstream path also degrades under the heaviest TCP workload, so not all observed degradation is attributable to HomeDNS.

The baseline JSON files under `benchmarks/baseline/sprint-02/` are the source of truth.

---

## 13. Extension Points

Future changes can be inserted around the current forwarding boundary.

```text
Receive
   |
   v
Validate
   |
   +--> Future client identification
   |
   +--> Future allow/block rule evaluation
   |
   +--> Future cache lookup
   |
   v
Forward upstream
   |
   +--> Future failover or load balancing
   |
   v
Future cache write
   |
   +--> Future asynchronous query event
   |
   v
Respond
```

These extension points are intentionally documented but not implemented in `v0.2.0`.
