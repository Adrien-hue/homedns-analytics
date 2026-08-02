# Runtime Monitoring Architecture

> **Scope:** Resource measurements collected during DNS benchmarks  
> **Production implementation:** Linux  
> **Sampling interval:** 1 second by default

---

## 1. Purpose

DNS performance must be evaluated together with resource cost.

A low-latency result is not acceptable if it causes excessive CPU use, memory growth, service restarts, thermal throttling, or instability on the Raspberry Pi.

The runtime monitoring subsystem collects process and system measurements while a benchmark suite is running and stores aggregated statistics in the canonical JSON report.

---

## 2. Collection Boundary

Resource collection wraps the measured benchmark suite.

```text
Before sample
     |
     v
Periodic sampler starts
     |
     v
Benchmark scenarios execute
     |
     v
Periodic sampler stops
     |
     v
After sample
     |
     v
Aggregate resource summary
```

The before and after samples describe boundary state.

Periodic samples describe behavior during the run.

The `samples_collected` field counts periodic samples only. Aggregate process statistics can include the before and after values as well, so the number of values used in an aggregate can be larger than `samples_collected`.

---

## 3. Monitored Processes

### 3.1 Benchmark process

The benchmark process PID is obtained from:

```go
os.Getpid()
```

This process runs the workload generator, progress reporting, resource collector, statistics, and report writer.

### 3.2 Target DNS service

On Linux, the benchmark command discovers the running DNS service by scanning `/proc`.

A candidate process must:

- not be the benchmark process itself;
- not contain the `benchmark` subcommand;
- use the same `--config` path;
- have a valid numeric PID.

If no unique target service can be identified, Linux resource collection fails instead of silently attributing metrics to the wrong process.

On non-Linux systems, target-process collection is disabled so local development benchmarks can still run.

---

## 4. Linux Data Sources

```text
/proc/stat
/proc/<pid>/stat
/proc/<pid>/status
/proc/loadavg
/proc/meminfo
/sys/class/thermal/thermal_zone0/temp
vcgencmd get_throttled
```

### 4.1 Aggregate CPU ticks

`/proc/stat` provides total system CPU ticks.

The collector reads the aggregate `cpu` line and sums its tick fields.

### 4.2 Process CPU ticks

`/proc/<pid>/stat` provides process user and system CPU ticks.

The process command field is enclosed in parentheses and may contain spaces, so parsing locates the final closing parenthesis before indexing the remaining fields.

### 4.3 Process RSS and threads

`/proc/<pid>/status` provides:

- `VmRSS` in kilobytes;
- `Threads` count.

RSS is converted to bytes before being stored.

### 4.4 Load average

`/proc/loadavg` provides the one-minute system load average.

Short quick benchmarks can legitimately report zero because Linux load averages are smoothed over time.

### 4.5 Available memory

`/proc/meminfo` provides `MemAvailable`.

The collector records available bytes rather than only free pages because `MemAvailable` better represents memory that can be allocated without swapping.

### 4.6 Temperature

`/sys/class/thermal/thermal_zone0/temp` reports Raspberry Pi temperature in millidegrees Celsius.

The collector converts it to Celsius.

### 4.7 Throttling

`vcgencmd get_throttled` reports Raspberry Pi power and thermal flags.

Example healthy state:

```text
throttled=0x0
```

The `homedns` user requires access to the Raspberry Pi video device interface. On the current host, this is provided through membership in the `video` group.

---

## 5. CPU Percentage Calculation

CPU percentage is calculated between two samples.

```text
process delta = current process ticks - previous process ticks
system delta  = current system ticks  - previous system ticks

CPU percent = process delta / system delta × 100
```

The first sample cannot calculate CPU percentage because no previous tick snapshot exists.

The collector guards against:

- missing previous samples;
- decreasing counters;
- zero system delta;
- process disappearance;
- malformed procfs values.

---

## 6. Raw Sample Model

Each sample contains:

### Benchmark process

- PID;
- CPU percent when available;
- RSS bytes;
- thread count.

### Target service

- PID;
- CPU percent when available;
- RSS bytes;
- thread count.

### System

- one-minute load average;
- available memory bytes;
- temperature Celsius;
- throttling state.

### Collection metadata

- UTC collection timestamp;
- non-fatal collection errors.

Collection errors do not automatically fail the DNS benchmark. They are retained in the report so missing metrics remain visible.

---

## 7. Summary Aggregation

Raw measurements are converted into the report's `ResourceSummary`.

### 7.1 Process CPU

- minimum;
- mean;
- peak.

### 7.2 Process RSS

- minimum;
- mean;
- peak.

### 7.3 Thread count

- minimum;
- mean;
- peak.

### 7.4 Load average

- before;
- mean during periodic samples;
- peak during periodic samples;
- after.

### 7.5 Available memory

- before;
- minimum during periodic samples;
- after.

### 7.6 Temperature

- before;
- mean during periodic samples;
- peak during periodic samples;
- after.

### 7.7 Throttling

- before state;
- whether any non-zero throttling state was observed;
- after state.

Empty collection-error slices are serialized as:

```json
"errors": []
```

rather than `null`, preserving a stable JSON shape.

---

## 8. Stability Detection

### 8.1 Process restart

The collector records PID before and after.

```text
same PID      -> restart_detected: false
different PID -> restart_detected: true
unknown PID   -> restart_detected: null
```

The target-service restart value is propagated into the benchmark summary.

### 8.2 Thermal throttling

Throttling is considered observed when any recognized state differs from a zero state such as:

```text
0
0x0
throttled=0
throttled=0x0
```

The result is propagated into:

```text
summary.stability.thermal_throttling_detected
```

### 8.3 Current unimplemented stability fields

The schema reserves fields for:

- service readiness after the run;
- memory-growth detection.

These fields remain `null` until the corresponding analysis is implemented.

---

## 9. Error Handling

The collector distinguishes benchmark errors from resource errors.

### Benchmark execution error

If the suite fails to execute, the report runner returns a benchmark-suite error.

### Collector execution error

If resource collection cannot wrap the run correctly, the report runner returns a resource-collection error.

### Individual metric error

If one metric cannot be read:

- the remaining metrics continue;
- the message is added to `resources.sampling.errors`;
- unavailable fields remain `null`.

This behavior allowed the throttling-permission issue to be identified without losing the latency and resource benchmark.

---

## 10. Platform Design

### Linux

Uses the full collector based on procfs, sysfs, and Raspberry Pi utilities.

### Non-Linux

Uses a no-op implementation.

This keeps:

- macOS development supported;
- Linux-specific code behind build tags;
- production collection explicit;
- cross-compilation verifiable.

Linux tests use build tags and can be compiled for ARM64 from macOS. They execute normally in Linux CI or on the Raspberry Pi.

---

## 11. Sprint 02 Baseline Interpretation

The endurance baseline demonstrated:

- benchmark-process mean CPU around 2.23%;
- DNS-service mean CPU around 1.57%;
- DNS-service peak CPU around 15.41%;
- DNS-service peak RSS around 14.31 MB;
- peak system load around 0.94;
- peak temperature around 52.62 °C;
- no process restart;
- no thermal throttling;
- no resource collection errors after permission correction.

This shows that heavy TCP timeout behavior was not caused by Raspberry Pi CPU saturation, memory exhaustion, or temperature throttling.

The baseline artifacts are stored under `benchmarks/baseline/sprint-02/`.

---

## 12. Security and Permissions

The DNS service runs as the unprivileged `homedns` user.

For throttling collection:

```text
homedns -> member of video group -> access to vcgencmd device
```

The benchmark report directories are owned by `homedns`. Administrative export may require `sudo` because `/opt/homedns/shared` is intentionally not broadly traversable.

Future operations documentation should include:

- required group membership;
- benchmark export procedure;
- safe directory permissions;
- validation after reboot.

---

## 13. Future Extensions

Potential additions include:

- health snapshots before and after benchmarks;
- service-ready detection;
- process restart count from systemd;
- automatic memory-growth analysis;
- disk I/O metrics;
- network-interface counters;
- CPU-frequency tracking;
- per-core CPU behavior;
- comparison of current samples with an established baseline;
- configurable sampling interval through the CLI;
- persistent time-series monitoring outside benchmark execution.

Continuous production monitoring is intentionally separate from the Sprint 02 benchmark-only collector.
