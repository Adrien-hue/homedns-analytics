# SPRINT-02 — DNS Forwarder MVP

> **Suggested sprint duration:** 4–7 days  
> **Milestone:** Milestone 1 — DNS MVP  
> **Target version:** `v0.2.0`  
> **Primary component:** Go DNS service

---

# 1. Sprint Goal

Build the first functional version of the HomeDNS DNS service: a lightweight Go application that receives DNS queries over UDP or TCP, forwards them to one configured upstream resolver, and returns the upstream response to the requesting client.

The result must run reliably on the Raspberry Pi 3B, remain simple enough to understand and maintain, and establish the first measurable DNS performance baseline for the project.

This sprint implements only the forwarding path:

```text
DNS client
    |
    | UDP or TCP query
    v
HomeDNS DNS forwarder
    |
    | upstream DNS query
    v
Configured upstream resolver
    |
    | upstream DNS response
    v
HomeDNS DNS forwarder
    |
    | response
    v
DNS client
```

No filtering, caching, persistence, analytics storage, dashboard integration, or advanced resolver strategy is included.

---

# 2. Context

Sprint 00 established the repository, documentation, Git workflow, CI foundations, development tooling, release conventions, and project structure.

Sprint 01 prepared the Raspberry Pi environment, including secure SSH access, system hardening, the dedicated application user, runtime directories, firewall configuration, and baseline resource measurements.

Sprint 02 is the first sprint that implements production application behavior. At the end of this sprint, HomeDNS must be capable of resolving real DNS requests through the custom Go service.

This sprint also establishes the first DNS-specific benchmark. Later sprints will compare their performance against this baseline when filtering, caching, persistence, and observability are introduced.

---

# 3. Dependencies

## Required completed work

- Sprint 00 — Project Foundation completed.
- Sprint 01 — Raspberry Pi Infrastructure completed.
- Git repository and branch protection operational.
- CI workflow capable of running repository checks.
- Go development environment available locally.
- Raspberry Pi reachable through SSH.
- `/opt/homedns` deployment structure available on the Raspberry Pi.
- Baseline system benchmark scripts available.

## Required external dependency

The DNS protocol implementation will use:

```text
github.com/miekg/dns
```

The project will use the stable v1 module line for the MVP.

HomeDNS will not implement DNS packet encoding, decoding, compression, or protocol primitives from scratch.

---

# 4. Sprint Principles

## 4.1 Small functional scope

Sprint 02 must remain intentionally small.

The objective is not to build a complete DNS appliance. The objective is to prove that HomeDNS can safely receive, forward, and return DNS traffic.

The core request path must remain easy to inspect:

```text
receive -> validate -> forward -> return -> observe
```

## 4.2 Hybrid learning approach

The project uses a hybrid approach:

- reuse a mature library for DNS protocol handling;
- implement HomeDNS-specific forwarding behavior;
- implement the project architecture, configuration, metrics, testing, benchmarking, packaging, and operations;
- implement filtering, caching, persistence, and analytics in later sprints.

## 4.3 One implementation per responsibility

Each responsibility must have one clear owner.

Examples:

- one configuration model;
- one upstream forwarding implementation;
- one DNS request handler;
- one metrics registry;
- one version source;
- one startup path;
- one shutdown path.

Duplicate helpers, parallel implementations, and unnecessary abstraction layers must be avoided.

## 4.4 Simple but evolvable

The service should not implement future features prematurely.

However, package boundaries and data structures should avoid blocking planned additions such as:

- multiple upstream resolvers;
- runtime configuration reload;
- filtering;
- caching;
- asynchronous logging;
- richer metrics;
- internal control endpoints.

These future capabilities should be documented, not implemented.

---

# 5. User Stories

## As a network client

I want to send a DNS request to HomeDNS and receive the same usable answer I would receive from the configured upstream resolver.

## As a developer

I want the forwarding logic separated into small, readable Go packages so that I can understand the implementation and safely extend it later.

## As an operator

I want to verify that the DNS service is running, identify its version, and inspect basic request and error counters.

## As a maintainer

I want automated tests and repeatable benchmarks so that future changes can be compared against the first DNS release.

## As a Raspberry Pi user

I want the DNS service to consume very little CPU and memory and to shut down cleanly without leaving open listeners or corrupted runtime state.

---

# 6. Scope

## 6.1 Included

Sprint 02 includes:

- Go module initialization;
- one DNS service binary;
- startup configuration loading;
- configuration defaults and validation;
- UDP DNS listener;
- TCP DNS listener;
- DNS request parsing through `miekg/dns`;
- forwarding to one configured upstream resolver;
- returning upstream responses to clients;
- upstream timeout handling;
- common DNS error handling;
- structured application logging;
- graceful shutdown;
- minimal internal health endpoint;
- basic in-memory runtime metrics;
- embedded build version information;
- unit tests;
- integration tests using a controlled local upstream server;
- Raspberry Pi validation;
- DNS latency and resource benchmarks;
- CI validation for the Go DNS component;
- sprint documentation and completion notes.

## 6.2 Explicitly excluded

Sprint 02 does not include:

- domain filtering;
- blacklists;
- whitelists;
- downloaded blocklists;
- DNS response caching;
- negative caching;
- SQLite;
- query history persistence;
- per-client analytics;
- asynchronous event logging;
- multiple upstream resolvers;
- upstream failover;
- upstream load balancing;
- retry strategies;
- runtime configuration reload;
- DNS-over-HTTPS;
- DNS-over-TLS;
- DNSSEC validation;
- a recursive resolver implemented by HomeDNS;
- authentication;
- management API integration;
- React dashboard integration;
- Prometheus exposition;
- production automatic deployment;
- router-wide DNS migration before validation is complete.

---

# 7. Functional Requirements

## FR-01 — UDP DNS listener

The service must accept DNS queries over UDP.

The listen address and port must be configurable.

The server must not assume port `53` during local development and automated tests.

## FR-02 — TCP DNS listener

The service must accept DNS queries over TCP on the same configured address and port.

UDP and TCP servers must run concurrently.

A failure to start either listener must prevent the service from reporting itself as ready.

## FR-03 — DNS request forwarding

For each accepted DNS request, HomeDNS must forward the request to the configured upstream DNS resolver.

The upstream address must include a host or IP address and port.

Example:

```text
1.1.1.1:53
```

## FR-04 — Transparent response return

HomeDNS must return the upstream DNS response to the original client without intentionally modifying valid answer records.

The forwarded response must preserve the request identifier expected by the client.

## FR-05 — Query-type independence

The forwarder must not contain special forwarding logic for individual common query types.

Valid DNS queries supported by the upstream resolver should be forwarded transparently, including:

- `A`;
- `AAAA`;
- `CNAME`;
- `MX`;
- `NS`;
- `PTR`;
- `TXT`;
- `SRV`;
- `SOA`;
- `CAA`;
- other standard query types accepted by `miekg/dns` and the upstream resolver.

The acceptance tests must explicitly verify at least `A`, `AAAA`, and a response containing a `CNAME` chain.

## FR-06 — Upstream timeout

The service must enforce a configurable upstream timeout.

When the upstream does not answer within the configured duration, HomeDNS must return a DNS `SERVFAIL` response when possible.

The timeout must not cause the process to crash or block future requests.

## FR-07 — Upstream error handling

Transport failures, malformed upstream responses, closed connections, and other forwarding failures must be handled safely.

When a valid client request cannot be completed, HomeDNS should return `SERVFAIL` when a response can be constructed.

The error must be recorded in structured logs and metrics.

## FR-08 — Invalid request safety

Malformed, unsupported, empty, or unexpected DNS requests must not crash the service.

The handler should return an appropriate DNS error response when possible. If a safe response cannot be generated, the request may be dropped after recording the failure.

## FR-09 — Graceful shutdown

The service must react to normal operating-system termination signals.

At minimum:

- `SIGINT`;
- `SIGTERM`.

Shutdown must:

1. stop accepting new health requests;
2. stop the UDP listener;
3. stop the TCP listener;
4. wait for shutdown operations within a bounded timeout;
5. emit a final structured log entry;
6. exit with a successful status when shutdown completes normally.

## FR-10 — Health endpoint

A minimal HTTP health endpoint must be available on a separately configured internal address.

Initial route:

```text
GET /health
```

The endpoint must be intended for localhost or controlled internal access only.

## FR-11 — Runtime metrics

The service must keep a small set of process-local counters and timing measurements.

These metrics are for validation and future observability design. They are not persisted in Sprint 02.

## FR-12 — Version output

The binary must support:

```bash
homedns-dns --version
```

The output must identify:

- application name;
- version;
- Git commit;
- build timestamp.

## FR-13 — Startup validation

The service must validate configuration before starting listeners.

Invalid configuration must produce:

- a clear error message;
- a non-zero exit code;
- no partially running DNS listener.

---

# 8. Non-Functional Requirements

## NFR-01 — Raspberry Pi compatibility

The service must build and run on Linux ARM64 for the Raspberry Pi 3B environment.

## NFR-02 — Low idle usage

With no incoming queries, the service should remain close to zero CPU usage.

Initial target:

```text
Idle CPU: approximately 0%, allowing normal measurement variation
```

## NFR-03 — Memory efficiency

Initial target for the DNS forwarder process:

```text
Idle RSS: below 20 MB
```

A lower measured result is preferred, but this value is a validation target rather than a strict architectural guarantee.

## NFR-04 — Processing overhead

The forwarder should add minimal latency compared with querying the same upstream resolver directly.

Initial target under normal LAN conditions:

```text
Median additional forwarding overhead: below 2 ms
95th percentile additional overhead: below 5 ms
```

These values must be measured and recorded. Environmental network variation must be documented.

## NFR-05 — Startup time

The service should become ready in less than one second on the Raspberry Pi under normal conditions.

## NFR-06 — Shutdown time

Normal graceful shutdown should complete within five seconds.

## NFR-07 — Concurrency safety

Concurrent UDP and TCP requests must not produce data races.

The Go race detector must be used in CI or local validation where practical.

## NFR-08 — Testability

The forwarding component must be testable against a controlled local upstream DNS server without requiring public internet access.

## NFR-09 — Clear logs

Logs must be understandable from `journald` or terminal output and must contain enough context to diagnose startup, forwarding, timeout, and shutdown failures.

## NFR-10 — No critical-path persistence

No file or database write may be required to answer a DNS request in Sprint 02.

---

# 9. Architecture Decisions

## AD-01 — Use `miekg/dns`

HomeDNS will use `github.com/miekg/dns` for DNS message parsing, encoding, server listeners, and client exchanges.

Reasoning:

- mature implementation;
- avoids reimplementing DNS wire-format details;
- supports UDP and TCP;
- suitable for an educational but production-oriented project;
- leaves project effort focused on forwarding behavior and future HomeDNS features.

## AD-02 — One service binary

Sprint 02 will produce one executable:

```text
homedns-dns
```

Reasoning:

- simple packaging;
- simple systemd supervision;
- simple version reporting;
- one release artifact for the DNS component;
- avoids premature CLI complexity.

The binary may gain subcommands in a future release only when multiple operational workflows genuinely require them.

## AD-03 — One configured upstream resolver

Sprint 02 supports exactly one upstream resolver.

Reasoning:

- easiest behavior to understand and test;
- no hidden retry or selection logic;
- establishes a clean baseline;
- multiple resolvers and failover can be designed later using measured behavior.

## AD-04 — Startup-only configuration

Configuration is loaded once during process startup and remains immutable for the lifetime of the process.

Runtime reload is deferred.

The configuration package should remain isolated so a future reload mechanism can be added without spreading file-reading logic across the service.

## AD-05 — Idiomatic Go package structure

The sprint will use small packages based on real responsibilities rather than enterprise-style layers.

The project will not introduce generic `usecase`, `repository`, `adapter`, or `port` layers for this simple forwarding path.

## AD-06 — Minimal interfaces

Interfaces should be introduced only where they provide immediate value, primarily for testing the request handler independently from the upstream transport.

Avoid defining interfaces before there is more than one useful implementation or a clear test boundary.

## AD-07 — In-memory metrics only

Metrics are stored in process memory and reset when the service restarts.

No metrics database or time-series system is added in this sprint.

## AD-08 — Health server separated from DNS listeners

The health endpoint runs on a dedicated HTTP listener.

It must not be exposed publicly by default and must not share the DNS port.

## AD-09 — Development port differs from production port

Local development and automated tests should use an unprivileged port such as `5353`.

Production may use port `53` after the required Linux capability or systemd binding strategy is explicitly configured.

The application must not require running as root solely for development.

## AD-10 — Benchmark direct and forwarded paths

Performance evaluation must compare:

```text
client -> upstream
```

against:

```text
client -> HomeDNS -> upstream
```

This is required to distinguish HomeDNS overhead from upstream and network latency.

---

# 10. Proposed Repository Structure

```text
dns/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── load.go
│   │   └── validate.go
│   │
│   ├── dnsserver/
│   │   ├── handler.go
│   │   ├── server.go
│   │   └── errors.go
│   │
│   ├── upstream/
│   │   ├── client.go
│   │   └── forwarder.go
│   │
│   ├── health/
│   │   ├── handler.go
│   │   └── server.go
│   │
│   ├── metrics/
│   │   └── metrics.go
│   │
│   └── version/
│       └── version.go
│
├── tests/
│   └── integration/
│       └── forwarder_test.go
│
├── config.example.yaml
├── go.mod
└── go.sum
```

This is a target structure, not a requirement to create empty files. Files should exist only when they contain a coherent responsibility.

---

# 11. Package Responsibilities

## `cmd/server`

Responsible for application assembly only.

Expected responsibilities:

- parse CLI flags;
- print version and exit when requested;
- load configuration;
- configure logging;
- construct metrics;
- construct upstream client;
- construct DNS handler;
- construct UDP and TCP servers;
- construct health server;
- start components;
- listen for termination signals;
- coordinate graceful shutdown;
- return meaningful process exit codes.

Business or forwarding logic must not be implemented in `main.go`.

## `internal/config`

Responsible for:

- configuration model;
- defaults;
- YAML loading;
- validation;
- human-readable validation errors.

No other package should read the configuration file directly.

## `internal/upstream`

Responsible for communication with the configured upstream resolver.

Expected behavior:

- accept a DNS request;
- select the transport required by the caller;
- enforce timeout;
- exchange the request with the upstream;
- return response, duration, and error;
- contain no filtering or caching logic.

## `internal/dnsserver`

Responsible for:

- UDP and TCP DNS listener lifecycle;
- DNS request handler;
- basic request validation;
- invoking the upstream forwarder;
- constructing safe error responses;
- writing the final response to the requesting client;
- recording metrics;
- structured request logging.

## `internal/health`

Responsible for:

- internal HTTP server lifecycle;
- `/health` response;
- readiness state;
- process version and uptime exposure;
- snapshot of lightweight metrics.

## `internal/metrics`

Responsible for:

- atomic counters;
- lightweight latency totals or histograms appropriate for Sprint 02;
- immutable snapshot generation;
- no file, network, or database output.

## `internal/version`

Responsible for the single source of build metadata:

- version;
- Git commit;
- build timestamp.

Development defaults must allow ordinary local builds without linker flags.

---

# 12. Configuration Contract

## 12.1 Configuration format

Sprint 02 will use a YAML configuration file.

Suggested default location for production:

```text
/opt/homedns/shared/config/dns.yaml
```

Suggested local example:

```text
dns/config.example.yaml
```

The path should be selectable with a CLI flag:

```bash
homedns-dns --config ./dns/config.example.yaml
```

## 12.2 Initial configuration schema

```yaml
server:
  listen_address: "127.0.0.1"
  port: 5353

upstream:
  address: "1.1.1.1:53"
  timeout: 3s

health:
  listen_address: "127.0.0.1"
  port: 8081

logging:
  level: "info"
  format: "json"

shutdown:
  timeout: 5s
```

## 12.3 Production example

```yaml
server:
  listen_address: "0.0.0.0"
  port: 53

upstream:
  address: "1.1.1.1:53"
  timeout: 3s

health:
  listen_address: "127.0.0.1"
  port: 8081

logging:
  level: "info"
  format: "json"

shutdown:
  timeout: 5s
```

## 12.4 Required validation

Configuration validation must reject:

- empty upstream address;
- upstream address without a valid port;
- invalid listen address;
- DNS port outside `1–65535`;
- health port outside `1–65535`;
- DNS and health listeners using the same address and port;
- zero or negative upstream timeout;
- zero or negative shutdown timeout;
- unsupported log level;
- unsupported log format.

## 12.5 Defaults

Reasonable defaults may be provided for local development, but production behavior must not silently choose an upstream resolver when the project intends explicit configuration.

Recommended approach:

- require upstream address;
- default DNS listen address to `127.0.0.1`;
- default DNS port to `5353`;
- default health address to `127.0.0.1`;
- default health port to `8081`;
- default upstream timeout to `3s`;
- default shutdown timeout to `5s`;
- default logging level to `info`;
- default logging format to `json`.

---

# 13. Command-Line Contract

Minimum supported commands:

```bash
homedns-dns --config <path>
homedns-dns --version
homedns-dns --help
```

Expected local usage:

```bash
go run ./cmd/server --config ./config.example.yaml
```

Expected binary usage:

```bash
./homedns-dns --config /opt/homedns/shared/config/dns.yaml
```

Version example:

```text
HomeDNS DNS
Version: v0.2.0
Commit: a3f8d21
Built: 2026-07-24T18:00:00Z
```

Development build example:

```text
HomeDNS DNS
Version: dev
Commit: unknown
Built: unknown
```

---

# 14. DNS Request Lifecycle

For each request:

```text
1. Receive DNS message over UDP or TCP
2. Record request start time
3. Increment total and protocol-specific counters
4. Validate that the message can be processed safely
5. Forward the message to the configured upstream resolver
6. Wait up to the configured timeout
7. Receive the upstream response
8. Restore or verify the client request identifier
9. Return the response to the client
10. Record response code, latency, and outcome
11. Emit structured request log
```

On failure:

```text
1. Classify timeout, transport error, malformed response, or local write error
2. Increment the corresponding error counter
3. Construct SERVFAIL when possible
4. Return the error response
5. Emit a structured error log
6. Keep the service available for future requests
```

---

# 15. UDP and TCP Behavior

## UDP

UDP is expected to handle most ordinary DNS traffic.

The service must:

- accept UDP requests;
- forward them using UDP by default;
- return valid upstream responses;
- handle truncated upstream responses according to the chosen forwarding strategy.

## TCP

TCP must be supported directly for clients that explicitly use TCP.

The service must:

- accept TCP connections;
- forward requests over TCP when the incoming request uses TCP;
- return the upstream response over the client TCP connection;
- clean up connections correctly.

## Truncated UDP response behavior

For the MVP, the preferred behavior is:

- when the upstream UDP response has the truncated flag, retry the same request to the same upstream over TCP;
- return the complete TCP response to the UDP client when it fits the client-supported message size;
- otherwise allow standard DNS truncation behavior.

This provides meaningful TCP fallback without adding multiple-upstream retry logic.

The exact behavior must be covered by an integration test using a controlled upstream server.

---

# 16. Error Handling Contract

## Startup errors

Examples:

- missing configuration file;
- malformed YAML;
- invalid address;
- unavailable port;
- failure to start health listener;
- failure to start UDP listener;
- failure to start TCP listener.

Expected behavior:

- log the error clearly;
- stop any component that already started;
- exit non-zero;
- never report ready state.

## Request errors

| Condition | Expected behavior |
|---|---|
| Upstream timeout | Return `SERVFAIL`, increment timeout counter |
| Upstream connection error | Return `SERVFAIL`, increment upstream error counter |
| Malformed client request | Return appropriate DNS error when safe, never panic |
| Empty question section | Reject safely, never panic |
| Response write failure | Log failure and increment response error counter |
| Upstream malformed response | Return `SERVFAIL` when possible |
| Panic in request handling | Recover at the request boundary, log, keep server running |

Panic recovery must not hide programming errors during tests. Tests should still make unexpected failures visible.

---

# 17. Structured Logging

## 17.1 Logging goals

Logs must support:

- startup diagnosis;
- request tracing at an operational level;
- upstream failure diagnosis;
- performance inspection;
- clean shutdown confirmation.

## 17.2 Log format

JSON is the preferred production format because it is structured and compatible with later parsing.

A human-readable text format may be supported for local development if it does not create duplicate logging implementations.

## 17.3 Startup log fields

Suggested fields:

```text
level
event
version
commit
build_time
dns_listen_address
health_listen_address
upstream
timeout
```

## 17.4 Request log fields

Suggested fields:

```text
level
event
client_address
protocol
request_id
query_name
query_type
query_class
upstream
latency_ms
rcode
response_bytes
outcome
```

## 17.5 Error log fields

Suggested fields:

```text
level
event
client_address
protocol
query_name
query_type
upstream
error_kind
error
latency_ms
```

## 17.6 Privacy limitation

Sprint 02 logs query names for engineering validation.

Before production-wide use, the project must review:

- retention;
- access permissions;
- log volume;
- privacy implications;
- whether successful request logs should use a lower log level or sampling.

No long-term query logging policy is finalized in this sprint.

---

# 18. Health Endpoint Contract

## Route

```text
GET /health
```

## Successful response

Suggested status code:

```text
200 OK
```

Suggested body:

```json
{
  "status": "ok",
  "ready": true,
  "version": "v0.2.0",
  "commit": "a3f8d21",
  "build_time": "2026-07-24T18:00:00Z",
  "uptime_seconds": 842,
  "dns": {
    "udp_listening": true,
    "tcp_listening": true,
    "listen_address": "0.0.0.0:53"
  },
  "upstream": {
    "address": "1.1.1.1:53"
  },
  "metrics": {
    "requests_total": 1457,
    "requests_udp": 1431,
    "requests_tcp": 26,
    "responses_noerror": 1392,
    "responses_nxdomain": 41,
    "responses_servfail": 24,
    "upstream_timeouts": 3,
    "upstream_errors": 2,
    "average_upstream_latency_ms": 16.4
  }
}
```

## Readiness behavior

The service must not report `ready: true` until:

- configuration is valid;
- UDP listener is running;
- TCP listener is running;
- health state is initialized.

The endpoint does not need to perform a live upstream DNS query for every request. Doing so would make health checks dependent on external network latency and would create unnecessary traffic.

A future readiness or upstream probe endpoint may be added later.

## Exposure

Default bind address:

```text
127.0.0.1
```

The health endpoint must not be exposed to the public internet.

---

# 19. Initial Runtime Metrics

The following metrics should be implemented if they can remain lightweight and clear:

## Counters

- `requests_total`;
- `requests_udp`;
- `requests_tcp`;
- `responses_noerror`;
- `responses_nxdomain`;
- `responses_servfail`;
- `responses_other`;
- `upstream_timeouts`;
- `upstream_errors`;
- `invalid_requests`;
- `response_write_errors`.

## Timing

- total upstream latency;
- number of measured upstream responses;
- calculated average upstream latency.

## Process information

- process start time;
- uptime;
- build version.

## Deferred metrics

The following are intentionally deferred:

- percentiles generated inside the running service;
- per-domain metrics;
- per-client metrics;
- Prometheus format;
- historical time series;
- goroutine dashboards;
- file descriptor metrics;
- OS CPU, RAM, temperature, or disk metrics;
- alert thresholds.

These may be measured externally by the benchmark scripts without expanding the runtime service.

---

# 20. Concurrency Model

`miekg/dns` will manage concurrent request handling using Go’s normal server behavior.

Sprint 02 will not introduce a custom worker pool.

Shared mutable state must be limited to:

- readiness state;
- atomic counters;
- atomic or synchronized latency totals;
- server lifecycle state.

Configuration must be immutable after startup.

No global mutable request state should exist.

All concurrency-sensitive code must pass race detection where supported.

---

# 21. Versioning and Build Metadata

## 21.1 Target version

The first DNS MVP release is planned as:

```text
v0.2.0
```

This represents the completion of Sprint 02 and Milestone 1.

## 21.2 Build variables

The version package should provide development defaults:

```go
Version   = "dev"
Commit    = "unknown"
BuildTime = "unknown"
```

Release builds should override them through linker flags.

Conceptual example:

```bash
go build \
  -ldflags "-X '<module>/internal/version.Version=v0.2.0' \
            -X '<module>/internal/version.Commit=$(git rev-parse --short HEAD)' \
            -X '<module>/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
  -o dist/homedns-dns \
  ./cmd/server
```

The exact module import path must match the repository.

## 21.3 Where version appears

Version information must appear in:

- `--version` output;
- startup logs;
- `/health` response;
- benchmark metadata;
- release artifact metadata where available.

---

# 22. Security Considerations

## 22.1 Least privilege

The DNS service should run as the dedicated `homedns` user.

It must not run as root for normal operation after the port-binding method is configured.

Possible production binding strategies include:

- granting only `CAP_NET_BIND_SERVICE` to the binary;
- applying the capability through systemd;
- using an alternative port during development.

The final production method should be documented when the service unit is activated.

## 22.2 Network exposure

DNS port `53` may be exposed to the LAN only.

The firewall must not expose the DNS service to an untrusted public interface.

The health endpoint should bind to localhost.

## 22.3 Open resolver risk

The service must not be configured as a publicly accessible open DNS resolver.

HomeDNS is designed for a home LAN or controlled lab network.

## 22.4 Input handling

DNS messages are untrusted input.

The service must:

- avoid panics from malformed packets;
- avoid unbounded allocations under normal library behavior;
- enforce upstream timeout;
- avoid logging binary packet content by default;
- avoid trusting client-provided names for filesystem operations.

## 22.5 Configuration permissions

Production configuration should be readable by the `homedns` service account and writable only by authorized administrators.

Suggested ownership:

```text
root:homedns
```

Suggested mode:

```text
0640
```

No secrets are expected in the Sprint 02 DNS configuration, but secure permissions should still be established.

---

# 23. Performance Considerations

## 23.1 Critical request path

The request path must avoid:

- database access;
- filesystem writes;
- synchronous analytics processing;
- external health checks;
- expensive string transformations;
- unnecessary DNS message copies;
- unbounded queues;
- per-request process creation.

## 23.2 Logging impact

Structured logging can affect DNS latency and storage.

For Sprint 02, request logging is useful for validation, but benchmark runs should record the configured log level and format.

If request-level `info` logging creates measurable overhead, the completion notes should document the result and recommend a later operational log policy.

## 23.3 Timeout selection

The default upstream timeout should be long enough for normal home-network conditions but short enough to avoid accumulating stalled requests.

Initial default:

```text
3 seconds
```

## 23.4 Metrics cost

Runtime metrics must use simple atomic or synchronized operations.

No per-request label map or high-cardinality metric structure should be introduced.

---

# 24. Testing Strategy

# 24.1 Unit tests

## Configuration tests

Test:

- valid configuration;
- default application;
- missing file;
- invalid YAML;
- invalid DNS port;
- invalid health port;
- invalid upstream address;
- invalid timeout;
- unsupported log level;
- listener collision.

## Version tests

Test:

- development defaults;
- formatted version output;
- health snapshot includes version metadata.

## Metrics tests

Test:

- counters increment correctly;
- protocol counters remain consistent with total requests;
- average latency calculation;
- concurrent increments;
- snapshot behavior.

## Upstream forwarder tests

Using a local test DNS server, test:

- successful UDP exchange;
- successful TCP exchange;
- timeout;
- connection error;
- malformed upstream behavior;
- truncated UDP response with TCP retry if implemented.

## DNS handler tests

Test:

- valid request forwarding;
- response ID preservation;
- `SERVFAIL` on upstream timeout;
- safe behavior for empty question section;
- safe behavior for malformed request objects where constructible;
- metrics updates;
- expected response code counters.

## Health handler tests

Test:

- HTTP status code;
- JSON content type;
- readiness false before listeners start;
- readiness true when both listeners are active;
- version fields;
- metrics fields;
- uptime increases.

---

# 24.2 Integration tests

Integration tests must not depend on public DNS availability.

They should start:

1. a controlled local upstream DNS server;
2. the HomeDNS DNS service on temporary or test ports;
3. test clients that query HomeDNS over UDP and TCP.

The local upstream should return deterministic answers.

Example test zones:

```text
example.test.        A       192.0.2.10
example.test.        AAAA    2001:db8::10
alias.test.          CNAME   example.test.
mail.test.           MX      10 mailhost.test.
```

Required integration scenarios:

- UDP `A` resolution;
- UDP `AAAA` resolution;
- response containing `CNAME`;
- TCP query;
- NXDOMAIN propagation;
- SERVFAIL propagation from upstream;
- upstream timeout transformed into local SERVFAIL;
- multiple concurrent requests;
- malformed or unexpected request safety;
- graceful shutdown;
- listener ports become reusable after shutdown;
- health endpoint reports ready while service is running.

---

# 24.3 Raspberry Pi validation

After local and CI validation, the ARM64 binary must be tested on the Raspberry Pi.

Required checks:

```bash
uname -m
./homedns-dns --version
```

Expected architecture:

```text
aarch64
```

Start the service on an unprivileged port first:

```bash
./homedns-dns --config ./dns-test.yaml
```

Validate UDP:

```bash
dig @127.0.0.1 -p 5353 example.com A
```

Validate AAAA:

```bash
dig @127.0.0.1 -p 5353 example.com AAAA
```

Validate TCP:

```bash
dig @127.0.0.1 -p 5353 example.com A +tcp
```

Validate health:

```bash
curl --fail http://127.0.0.1:8081/health
```

Validate shutdown:

```bash
kill -TERM <pid>
```

Confirm:

- process exits cleanly;
- ports close;
- restart succeeds;
- no panic appears in logs.

Only after this validation should port `53` and LAN clients be tested.

---

# 25. Benchmark Plan

## 25.1 Purpose

The Sprint 02 benchmark establishes the first DNS application baseline.

It must answer:

- Does the forwarder work consistently?
- How much latency does it add?
- How many requests can it process?
- How much CPU and memory does it consume?
- Does TCP behave differently from UDP?
- Does logging materially affect performance?
- Are there timeouts or errors under expected home-network load?

## 25.2 Comparison paths

### Direct path

```text
benchmark client -> configured upstream
```

### HomeDNS path

```text
benchmark client -> HomeDNS -> configured upstream
```

The calculated overhead is:

```text
HomeDNS latency - direct upstream latency
```

Because network latency varies, results should use repeated samples and percentile summaries rather than a single query.

## 25.3 Required metrics

### DNS behavior

- total queries attempted;
- successful queries;
- failed queries;
- timeout count;
- response-code distribution;
- UDP query count;
- TCP query count.

### Latency

- minimum;
- mean;
- median;
- p95;
- p99;
- maximum;
- direct upstream values;
- HomeDNS values;
- calculated overhead values.

### Throughput

- sequential queries per second;
- controlled concurrent queries per second;
- success rate under load.

### Process resources

- idle RSS;
- peak RSS;
- idle CPU;
- CPU during load;
- process threads;
- goroutine count if exposed through a temporary benchmark mechanism or captured internally;
- open file descriptors;
- startup time;
- shutdown time.

### Raspberry Pi system context

- device hostname;
- architecture;
- operating-system version;
- kernel version;
- Go version;
- HomeDNS version;
- Git commit;
- upstream resolver;
- timestamp;
- temperature before and after;
- system load;
- available memory;
- storage availability;
- throttling status when available.

## 25.4 Workload

Initial benchmark workload should remain safe for the Raspberry Pi and home network.

Suggested phases:

### Phase 1 — Warm-up

```text
100 queries
```

Purpose:

- confirm connectivity;
- allow DNS and network state to stabilize;
- exclude startup anomalies from measured samples.

### Phase 2 — Sequential latency

```text
1,000 UDP queries
```

Use a controlled domain list and compare direct vs forwarded queries.

### Phase 3 — TCP latency

```text
100 TCP queries
```

### Phase 4 — Moderate concurrency

```text
1,000 total queries
10 concurrent workers
```

### Phase 5 — Short sustained load

```text
30–60 seconds
```

Use a bounded target rate appropriate for the Pi.

The benchmark must not be treated as a denial-of-service test.

## 25.5 Domain set

Use a documented domain set with a mixture of:

- `A`;
- `AAAA`;
- CNAME-based responses;
- NXDOMAIN queries using randomized names under a controlled invalid or test domain;
- repeated domains;
- unique domains.

Because Sprint 02 has no HomeDNS cache, repeated queries are still useful for checking upstream and OS-level effects, but they must not be described as HomeDNS cache performance.

## 25.6 Output structure

Suggested output:

```text
benchmarks/
└── dns/
    ├── README.md
    ├── runs/
    │   └── 2026-07-24_18-30-00/
    │       ├── metadata.env
    │       ├── direct-latency.csv
    │       ├── forwarded-latency.csv
    │       ├── tcp-latency.csv
    │       ├── load-results.csv
    │       ├── resources.txt
    │       ├── health-before.json
    │       ├── health-after.json
    │       └── summary.md
    └── latest -> runs/2026-07-24_18-30-00
```

The `latest` symlink should only be used if it remains portable and safe across the supported environments. Otherwise, a text pointer or Make output may be used.

## 25.7 Benchmark command

Recommended root command:

```bash
make benchmark-dns
```

Potential supporting commands:

```bash
make benchmark-dns-local
make benchmark-dns-rpi
make benchmark-dns-report
```

Only targets that have a clear and working implementation should be added.

## 25.8 Benchmark acceptance

The sprint does not fail merely because an aspirational performance target is missed.

It fails when:

- performance is not measured;
- methodology is not documented;
- errors are hidden;
- results cannot be reproduced;
- severe regressions or instability are ignored.

Measured limitations must be recorded honestly in the sprint completion notes.

---

# 26. Suggested Implementation Sequence

## Phase 1 — Go module and skeleton

- create the Sprint 02 branch;
- initialize the Go module under `dns/`;
- add `miekg/dns`;
- add the package structure;
- add a minimal `main.go`;
- add local build and test Make targets;
- confirm CI can run `go test ./...`.

## Phase 2 — Version support

- create the version package;
- add development defaults;
- implement `--version`;
- add linker-flag build support;
- test version formatting.

## Phase 3 — Configuration

- define the configuration model;
- implement YAML loading;
- implement defaults;
- implement validation;
- add `config.example.yaml`;
- add unit tests.

## Phase 4 — Controlled upstream test server

- build a reusable local DNS test server helper;
- define deterministic records;
- support delayed responses;
- support NXDOMAIN and SERVFAIL cases;
- support UDP and TCP.

Creating the test upstream early prevents internet-dependent development.

## Phase 5 — Upstream client

- implement UDP exchange;
- implement TCP exchange;
- enforce timeout;
- return latency and errors;
- test all transport and timeout cases.

## Phase 6 — DNS handler

- validate requests safely;
- call the upstream client;
- preserve request identity;
- construct SERVFAIL responses;
- record basic counters;
- add structured request logs;
- test handler behavior.

## Phase 7 — DNS listener lifecycle

- start UDP listener;
- start TCP listener;
- expose readiness state;
- coordinate startup failures;
- implement shutdown;
- add lifecycle integration tests.

## Phase 8 — Health endpoint

- add internal HTTP listener;
- expose version, readiness, uptime, and metrics;
- add endpoint tests;
- bind to localhost by default.

## Phase 9 — Local end-to-end validation

- run HomeDNS locally on port `5353`;
- validate with `dig` over UDP and TCP;
- validate timeout behavior;
- validate shutdown;
- run race tests;
- record local results.

## Phase 10 — Benchmark tooling

- implement direct and forwarded measurements;
- capture metadata;
- capture process resources;
- generate summary output;
- integrate with the existing benchmark orchestrator;
- add `make benchmark-dns`.

## Phase 11 — Raspberry Pi validation

- build or cross-compile ARM64 binary;
- transfer to the Pi;
- run on port `5353` first;
- validate health and DNS behavior;
- run benchmark;
- record resource and temperature results.

## Phase 12 — Port 53 and service validation

- configure the chosen least-privilege port-binding method;
- activate or update `homedns-dns.service`;
- validate restart and reboot behavior;
- test from one selected LAN client;
- do not switch the whole network yet unless recovery is prepared.

## Phase 13 — Documentation and release

- update README commands;
- update Make help;
- record benchmark baseline;
- complete sprint notes;
- create pull request to `develop`;
- validate CI;
- merge according to Git Flow Lite;
- prepare `v0.2.0` milestone release when the project workflow requires it.

---

# 27. Makefile Scope

Suggested root targets:

```text
make dns-format
make dns-lint
make dns-test
make dns-race
make dns-build
make dns-run
make dns-ci
make benchmark-dns
```

Existing generic targets should delegate rather than duplicate command definitions.

Example principle:

```text
make test
    -> invokes component test targets

make dns-test
    -> contains the Go DNS test command
```

There should be one authoritative command for each operation.

Potential commands:

```bash
cd dns && gofmt -w .
cd dns && go vet ./...
cd dns && go test ./...
cd dns && go test -race ./...
cd dns && go build ./cmd/server
```

The exact lint command should match the repository’s chosen Go tooling.

---

# 28. CI Requirements

Every Sprint 02 pull request must validate at least:

```text
Go formatting check
Go vet
Go unit tests
Go integration tests
Go build
ARM64 build or cross-compilation
```

Race detection should run where execution time and CI architecture make it practical.

The integration test suite must use the local controlled upstream server and must not depend on external DNS availability.

Suggested local equivalent:

```bash
make dns-ci
```

A failing DNS test or build must block merge.

---

# 29. Validation Commands

## Local development

```bash
cd dns
go mod tidy
go test ./...
go test -race ./...
go vet ./...
go build -o ../dist/homedns-dns ./cmd/server
```

## Start local server

```bash
./dist/homedns-dns --config ./dns/config.example.yaml
```

## UDP query

```bash
dig @127.0.0.1 -p 5353 example.com A
```

## AAAA query

```bash
dig @127.0.0.1 -p 5353 example.com AAAA
```

## TCP query

```bash
dig @127.0.0.1 -p 5353 example.com A +tcp
```

## CNAME response

```bash
dig @127.0.0.1 -p 5353 www.github.com CNAME
```

The exact public test domain may change over time. Deterministic automated tests must use the local test upstream.

## Health endpoint

```bash
curl --fail --silent http://127.0.0.1:8081/health
```

## Version

```bash
./dist/homedns-dns --version
```

## Benchmark

```bash
make benchmark-dns
```

## Raspberry Pi process inspection

```bash
ps -o pid,ppid,user,%cpu,%mem,rss,vsz,etime,cmd -C homedns-dns
```

## Listening ports

```bash
sudo ss -lntup | grep -E '(:53|:5353|:8081)'
```

## Service logs

```bash
journalctl -u homedns-dns.service --since today
```

## Service state

```bash
systemctl status homedns-dns.service
```

---

# 30. Deliverables

Sprint 02 must produce:

- initialized Go DNS module;
- `homedns-dns` binary;
- UDP DNS forwarding;
- TCP DNS forwarding;
- one configurable upstream resolver;
- startup configuration file and example;
- configuration validation;
- structured logs;
- timeout and error handling;
- graceful shutdown;
- `/health` endpoint;
- lightweight runtime metrics;
- embedded version information;
- unit test suite;
- deterministic integration test suite;
- ARM64 build validation;
- Raspberry Pi functional validation;
- DNS benchmark script or orchestrator integration;
- Sprint 02 benchmark report;
- updated Make targets;
- updated CI checks;
- updated documentation;
- recorded known limitations.

---

# 31. Acceptance Criteria

## Functional acceptance

- [ ] The binary starts with a valid configuration.
- [ ] The binary rejects invalid configuration and exits non-zero.
- [ ] UDP DNS queries are accepted and forwarded.
- [ ] TCP DNS queries are accepted and forwarded.
- [ ] `A` queries resolve through HomeDNS.
- [ ] `AAAA` queries resolve through HomeDNS.
- [ ] A response containing a `CNAME` chain is returned correctly.
- [ ] NXDOMAIN responses are propagated correctly.
- [ ] Upstream SERVFAIL responses are propagated correctly.
- [ ] Upstream timeout returns SERVFAIL when possible.
- [ ] Invalid requests do not crash the service.
- [ ] Concurrent requests do not crash or corrupt service state.
- [ ] Graceful shutdown closes UDP, TCP, and health listeners.
- [ ] Restart succeeds after graceful shutdown.

## Health and version acceptance

- [ ] `--version` returns version, commit, and build time.
- [ ] Startup logs include version information.
- [ ] `/health` returns valid JSON.
- [ ] `/health` reports readiness.
- [ ] `/health` reports uptime.
- [ ] `/health` reports UDP and TCP listener state.
- [ ] `/health` reports basic request and error metrics.
- [ ] Health endpoint binds to localhost by default.

## Quality acceptance

- [ ] Go formatting passes.
- [ ] `go vet` passes.
- [ ] Unit tests pass.
- [ ] Integration tests pass without public internet access.
- [ ] Race detection passes where executed.
- [ ] CI builds the DNS service.
- [ ] ARM64 build succeeds.
- [ ] No secret or private configuration is committed.
- [ ] Package responsibilities remain clear and non-duplicated.

## Raspberry Pi acceptance

- [ ] ARM64 binary runs on the Raspberry Pi.
- [ ] Idle CPU is recorded.
- [ ] Idle RSS is recorded.
- [ ] Startup time is recorded.
- [ ] UDP and TCP queries succeed on the Pi.
- [ ] Health endpoint succeeds on the Pi.
- [ ] Graceful shutdown succeeds on the Pi.
- [ ] Service can restart after failure or manual stop.
- [ ] Temperature and throttling state are recorded during benchmark validation.

## Benchmark acceptance

- [ ] Direct upstream latency is measured.
- [ ] HomeDNS forwarded latency is measured.
- [ ] Forwarding overhead is calculated.
- [ ] UDP and TCP results are recorded.
- [ ] Moderate concurrent load is tested.
- [ ] CPU and memory are recorded.
- [ ] Errors and timeouts are recorded.
- [ ] Benchmark metadata includes version and commit.
- [ ] Results are stored in a timestamped run directory.
- [ ] A readable summary report is generated.

---

# 32. Exit Criteria

Sprint 02 is complete when:

- [ ] all mandatory deliverables exist;
- [ ] all functional acceptance criteria pass;
- [ ] automated DNS tests pass in CI;
- [ ] ARM64 build succeeds;
- [ ] the forwarder runs successfully on the Raspberry Pi;
- [ ] `dig` resolves `A`, `AAAA`, and TCP queries through HomeDNS;
- [ ] upstream timeout behavior is validated;
- [ ] the health endpoint reports the running version and basic metrics;
- [ ] the first DNS benchmark report is committed or archived according to repository policy;
- [ ] measured performance and resource usage are documented;
- [ ] known limitations are documented;
- [ ] incomplete optional tasks are moved to the backlog;
- [ ] Sprint 02 completion notes are filled in;
- [ ] the project is ready for Sprint 03 — Filtering Engine.

---

# 33. Known Limitations

The Sprint 02 release intentionally has the following limitations:

- one upstream resolver only;
- no upstream fallback;
- no retry across multiple resolvers;
- no load balancing;
- no DNS cache;
- no filtering;
- no blacklist or whitelist;
- no database;
- no query history;
- no historical metrics;
- no analytics dashboard;
- no per-client identity;
- no runtime configuration reload;
- no authenticated internal control API;
- no encrypted upstream DNS;
- no DNSSEC validation performed by HomeDNS;
- no high availability;
- no secondary HomeDNS node;
- no rate limiting;
- no persistent metrics;
- request logs may be too verbose for long-term production use;
- a service restart resets all counters;
- public router-wide adoption is not required for sprint completion.

These limitations are deliberate and must not be treated as missing implementation unless the sprint scope is formally changed.

---

# 34. Risks and Mitigations

## Risk — DNS outage during testing

**Description:** Misconfiguration can prevent clients from resolving domains.

**Mitigation:**

- begin on port `5353`;
- test with explicit `dig @server -p 5353` commands;
- test from one selected client before changing router DNS;
- keep the current resolver configuration documented;
- maintain a recovery command and SSH access.

## Risk — Running as root for port 53

**Description:** Binding to port `53` may encourage running the entire process as root.

**Mitigation:**

- use an unprivileged port during development;
- use systemd capability configuration or `CAP_NET_BIND_SERVICE` for production;
- run the service as `homedns`.

## Risk — Public open resolver

**Description:** Incorrect firewall or router exposure could make HomeDNS publicly reachable.

**Mitigation:**

- LAN-only firewall rule;
- no router port forwarding;
- verify external exposure is absent;
- document intended interfaces.

## Risk — Internet-dependent tests

**Description:** Tests using public DNS can be flaky or misleading.

**Mitigation:**

- use a controlled local upstream server for automated tests;
- reserve public resolver checks for manual validation only.

## Risk — Logging overhead

**Description:** Per-request JSON logs may affect latency and microSD wear.

**Mitigation:**

- measure with benchmark;
- make log level configurable;
- avoid file logging directly from the application;
- use journald;
- revisit request-log policy in observability sprint.

## Risk — Premature architecture complexity

**Description:** Adding interfaces and layers for future features could make the first Go sprint difficult to understand.

**Mitigation:**

- create packages around current responsibilities only;
- introduce abstractions only for actual test boundaries;
- prefer concrete types;
- review exported APIs during pull request.

## Risk — Misleading benchmark results

**Description:** Network and upstream variation can hide or exaggerate HomeDNS overhead.

**Mitigation:**

- measure direct and forwarded paths close together in time;
- use repeated samples;
- record percentiles;
- record environment metadata;
- avoid claiming universal performance from one run.

## Risk — Raspberry Pi resource regression

**Description:** Logging, metrics, or concurrency behavior could consume unexpected resources.

**Mitigation:**

- record idle and load resources;
- inspect goroutines and descriptors;
- use race tests;
- keep metrics bounded;
- compare with Sprint 01 baseline.

---

# 35. Suggested GitHub Issues

Suggested issue breakdown:

1. Initialize Go DNS module and project structure
2. Add build version metadata and CLI flags
3. Implement DNS configuration loading and validation
4. Create deterministic local upstream DNS test server
5. Implement upstream UDP and TCP forwarding client
6. Implement DNS request handler and SERVFAIL behavior
7. Implement UDP and TCP server lifecycle
8. Add structured DNS logging
9. Add runtime metrics and internal health endpoint
10. Add graceful shutdown and signal handling
11. Add unit and integration tests
12. Add Go CI and ARM64 build validation
13. Add DNS benchmark workflow and report generation
14. Validate DNS forwarder on Raspberry Pi
15. Configure least-privilege port 53 service execution
16. Complete Sprint 02 documentation and results

Issues may be combined when the work remains reviewable. Avoid creating issues that are too small to produce a meaningful change.

---

# 36. Suggested Branches

Primary sprint branch if the work is handled as one coordinated feature:

```text
feature/dns-forwarder-mvp
```

Preferred smaller feature branches from `develop`:

```text
feature/dns-project-foundation
feature/dns-configuration
feature/dns-upstream-forwarder
feature/dns-server-lifecycle
feature/dns-health-metrics
feature/dns-benchmarks
```

Each branch should remain short-lived and merge into `develop` through a pull request.

---

# 37. Suggested Commits

Examples:

```text
feat(dns): initialize Go DNS service
feat(dns): add version metadata and CLI flags
feat(dns): load and validate service configuration
feat(dns): implement upstream DNS forwarding
feat(dns): serve UDP and TCP requests
feat(dns): add graceful shutdown
feat(dns): expose internal health metrics
test(dns): add deterministic forwarding integration tests
build(dns): add ARM64 binary target
perf(dns): add forwarding benchmark workflow
ci(dns): validate Go service in pull requests
docs(sprint): complete Sprint 02 specification
```

Commits should remain focused and use the project’s Conventional Commit rules.

---

# 38. Definition of Done

A Sprint 02 task is done when:

- implementation is complete;
- behavior is covered by appropriate tests;
- formatting and linting pass;
- errors are handled explicitly;
- package ownership remains clear;
- configuration changes update the example file;
- logs provide enough diagnostic context;
- security impact has been considered;
- performance impact has been considered;
- documentation is updated;
- the pull request passes CI;
- reviewer feedback is resolved.

Sprint 02 is done when:

- all mandatory deliverables exist;
- acceptance criteria pass;
- Raspberry Pi validation succeeds;
- benchmark results are recorded;
- limitations and deviations are documented;
- remaining work is moved explicitly to the backlog;
- the service is ready to become the foundation for Sprint 03 filtering.

---

# 39. Completion Notes

Complete this section at the end of the sprint.

## Final version

```text
Version:
Git commit:
Release date:
```

## Implemented scope

```text
- 
- 
- 
```

## Deviations from specification

```text
- 
```

## Raspberry Pi validation results

```text
Architecture:
Idle CPU:
Idle RSS:
Startup time:
Shutdown time:
Temperature before benchmark:
Temperature after benchmark:
Throttling status:
```

## DNS benchmark summary

```text
Upstream resolver:
Direct median latency:
Forwarded median latency:
Median forwarding overhead:
Forwarded p95 latency:
Forwarded p99 latency:
Sequential QPS:
Concurrent QPS:
Success rate:
Timeout count:
Peak RSS:
Peak CPU:
```

## Known issues

```text
- 
```

## Backlog items created

```text
- 
```

## Final decision

```text
[ ] Sprint accepted
[ ] Sprint accepted with documented limitations
[ ] Sprint not accepted
```

---

# 40. Next Sprint

Sprint 03 will build the filtering engine on top of the forwarding service.

Planned additions include:

- normalized domain rules;
- explicit whitelist priority;
- user blacklist;
- downloaded blocklist parsing;
- exact and suffix matching;
- in-memory rule loading;
- safe runtime rule reload;
- blocking response policy;
- filtering tests and benchmarks.

Sprint 03 must preserve the forwarding behavior, reliability, and benchmark visibility established in Sprint 02.
