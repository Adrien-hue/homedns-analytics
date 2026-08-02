# Sprint 02 --- DNS Forwarder MVP

## Location

Place this file in:

    docs/sprints/SPRINT-02.md

## Sprint Goal

Deliver the first production-ready HomeDNS DNS forwarder running on the
Raspberry Pi with:

-   DNS forwarding over UDP and TCP
-   Health endpoint
-   Structured logging
-   Benchmark framework
-   Automated deployment
-   Runtime resource monitoring
-   Safe release and rollback mechanism

## Deliverables

### DNS Server

-   UDP/TCP DNS forwarding
-   Configurable upstream resolver
-   Configurable timeouts
-   Graceful shutdown

### Health & Metrics

-   `/health` endpoint
-   Runtime metrics
-   Version/build metadata

### Benchmark Framework

-   Quick profile
-   Validation profile
-   Endurance profile
-   JSON report generation
-   Pass / Degraded / Failed evaluation
-   Resource sampling (CPU, RSS, threads, temperature, throttling)

### Deployment

-   Linux ARM64 release packaging
-   Atomic release activation
-   Automatic rollback
-   Release validation
-   Shared benchmark directory

## Baseline Results (v0.2.0-rc2)

  Profile      Status       Attempted   Successful   Failed   Timeouts
  ------------ ---------- ----------- ------------ -------- ----------
  Quick        Passed             800          800        0          0
  Validation   Degraded         8,000        7,986       14         13
  Endurance    Degraded       136,000      134,783    1,217      1,208

### Hardware observations

-   No service restart detected
-   No thermal throttling
-   CPU and memory remained stable
-   Peak temperature ≈ 52.6 °C
-   Raspberry Pi 3B remained responsive throughout endurance testing

## Lessons Learned

-   End-to-end deployment validation is essential.
-   Automatic rollback significantly reduces deployment risk.
-   Runtime resource collection provides valuable insight beyond latency
    alone.
-   TCP concurrent scenarios reveal bottlenecks that quick tests do not
    expose.

## Technical Debt

-   Improve TCP concurrent performance.
-   Introduce DNS caching.
-   Add blocklists and filtering.
-   Expose benchmark history through the future dashboard.

## Sprint Outcome

Sprint 02 objectives were achieved. HomeDNS now provides a deployable
DNS forwarder with reproducible benchmarking and a stable performance
baseline for future optimization.
