# Benchmark System Architecture

> **Introduced:** Sprint 02  
> **Command:** `homedns-dns benchmark`  
> **Canonical report schema:** `2.0`

---

## 1. Purpose

The benchmark system measures the behavior and cost of the HomeDNS DNS forwarder on real hardware.

It answers four questions:

1. Is HomeDNS reliable?
2. What latency overhead does forwarding introduce?
3. How much throughput can the system sustain?
4. What CPU, memory, and thermal cost accompanies the workload?

The benchmark framework is part of the product architecture. Future DNS features must be evaluated against the Sprint 02 baseline rather than optimized from intuition alone.

---

## 2. High-Level Design

```text
CLI
 |
 v
Application benchmark orchestration
 |
 v
Benchmark service
 |
 v
Report runner
 |
 +--------------------+----------------------+
 |                    |                      |
 v                    v                      v
Suite runner     Resource collector      Report builder
 |                    |                      |
 v                    v                      v
DNS scenarios     Linux samples          Classification
                                            |
                                            v
                                      JSON report writer
```

---

## 3. Command Boundary

The production binary exposes two operating modes:

```text
homedns-dns --config <path>
homedns-dns benchmark --config <path> --profile <name>
```

The benchmark subcommand uses the same configuration model and release metadata as the DNS service.

This avoids creating a separate benchmark binary with different assumptions or dependencies.

The CLI is responsible for:

- parsing profile and override flags;
- loading the DNS configuration;
- displaying the effective workload;
- attaching the terminal progress reporter;
- invoking application-level benchmark orchestration;
- printing the final report location and high-level result.

---

## 4. Benchmark Profiles

Sprint 02 defines three canonical profiles.

| Profile | Queries per path | Warmup per path | Concurrent workers | Intended use |
|---|---:|---:|---:|---|
| `quick` | 100 | 10 | 10 | Fast functional and smoke validation |
| `validation` | 1,000 | 25 | 10 | Release-level reliability and performance check |
| `endurance` | 17,000 | 50 | 10 | Sustained workload and stability baseline |

Each scenario contains a direct path and a forwarded path. Because four scenarios are executed, the total measured requests are:

```text
queries per path × 2 paths × 4 scenarios
```

The profile can be customized through CLI flags. Customized runs are explicitly marked in the report.

---

## 5. Scenario Matrix

The benchmark suite runs four scenarios in a stable order.

| Scenario | Transport | Execution model |
|---|---|---|
| `udp-sequential` | UDP | One request at a time |
| `udp-concurrent` | UDP | Configured worker pool |
| `tcp-sequential` | TCP | One request at a time |
| `tcp-concurrent` | TCP | Configured worker pool |

Each scenario compares:

### Direct path

```text
Benchmark process -> configured upstream resolver
```

### Forwarded path

```text
Benchmark process -> HomeDNS -> configured upstream resolver
```

This design distinguishes upstream or network limitations from HomeDNS-specific overhead.

---

## 6. Query Workload

The default query names are:

- `example.com.`;
- `cloudflare.com.`;
- `google.com.`;
- `github.com.`.

The default query type is `A`. `AAAA` can be selected through the CLI.

Names are normalized to fully qualified domain names before execution.

The workload rotates through the configured names so a run does not measure only one domain.

---

## 7. Warmup and Measurement

Warmup requests run before each measured path.

Purpose:

- reduce first-request effects;
- allow network state to stabilize;
- avoid mixing initialization cost into measured request statistics.

Warmup results are displayed but excluded from the final measured request counts and latency statistics.

---

## 8. Execution Components

### 8.1 Suite runner

The suite runner:

- resolves the ordered scenario definitions;
- executes each scenario;
- propagates progress events;
- returns scenario results.

### 8.2 Scenario runner

A scenario runner executes the direct and forwarded paths for one transport and concurrency model.

It records:

- attempted requests;
- successful requests;
- failures;
- timeouts;
- non-timeout failures;
- elapsed duration;
- successful-request latencies;
- attempted and successful throughput.

### 8.3 Progress reporter

The benchmark domain emits structured progress events.

The terminal implementation renders:

- benchmark start;
- scenario number and name;
- warmup completion;
- path start and completion;
- progress bar;
- attempted and successful QPS;
- failures and timeouts;
- phase duration;
- benchmark ETA;
- scenario comparison.

The benchmark logic does not depend on terminal formatting.

### 8.4 Report runner

The report runner coordinates:

1. report metadata initialization;
2. optional resource collection;
3. suite execution;
4. scenario classification;
5. report completion timestamps;
6. summary generation;
7. stability-field propagation.

### 8.5 Benchmark service

The benchmark service:

- creates a unique report ID;
- invokes the report runner;
- writes the canonical JSON report;
- returns the report and persisted path.

---

## 9. Status Classification

Each direct and forwarded path is evaluated against failure and timeout thresholds.

Default thresholds:

| Metric | Degraded | Failed |
|---|---:|---:|
| Failure percentage | 1% | 5% |
| Timeout percentage | 1% | 5% |

Scenario status can be:

- `passed`;
- `degraded`;
- `failed`;
- `invalid`.

The overall report status is derived from the most severe scenario status.

Status reasons include:

- code;
- scenario;
- path;
- observed value;
- threshold;
- human-readable message.

A degraded result is preserved as a valid baseline. It represents measured system behavior rather than a test harness failure.

---

## 10. Canonical JSON Report

The report schema is versioned independently from the application version.

Main sections:

```text
schema_version
report
benchmark_binary
target_service
environment
configuration
execution
resources
scenarios
summary
```

### `report`

- report ID;
- creation time;
- final status;
- status reasons;
- notes.

### `benchmark_binary`

- project;
- component;
- version;
- Git commit;
- Git branch and dirty state when known;
- Go version;
- build timestamp.

### `target_service`

- DNS and health addresses;
- service metadata when available;
- PID and restart-related state where supported.

### `configuration`

- profile;
- customization flag;
- addresses;
- query names and type;
- warmup and measured counts;
- concurrency;
- timeout;
- resource-sampling interval;
- scenario order;
- classification thresholds.

### `resources`

- sampling boundaries and errors;
- benchmark-process statistics;
- target-service statistics;
- system statistics.

### `scenarios`

Detailed direct and forwarded results for every scenario.

### `summary`

- aggregate requests and rates;
- aggregate throughput;
- direct and forwarded reliability;
- scenario-status groups;
- worst scenario;
- stability indicators.

---

## 11. Resource Collection Integration

Resource collection wraps suite execution.

```text
Collect before sample
        |
        v
Start periodic sampler
        |
        v
Run benchmark suite
        |
        v
Stop sampler
        |
        v
Collect after sample
        |
        v
Build resource summary
```

The benchmark process PID is `os.Getpid()`.

On Linux, the target DNS service PID is discovered by scanning `/proc` for the `homedns-dns` process using the same configuration path while excluding the benchmark subcommand.

On non-Linux platforms, live resource collection is disabled without preventing local benchmark execution.

See [`runtime-monitoring.md`](runtime-monitoring.md).

---

## 12. Baseline Artifacts

The official Sprint 02 baseline is stored under:

```text
benchmarks/baseline/sprint-02/
```

It contains:

- `RELEASE`;
- `health-before.json`;
- `health-after.json`;
- `quick.json`;
- `validation.json`;
- `endurance.json`.

All three benchmark reports reference:

```text
Version: v0.2.0-rc2
Commit:  a0e40004f48e
```

Baseline summary:

| Profile | Status | Attempted | Successful | Failed | Timeouts |
|---|---|---:|---:|---:|---:|
| Quick | Passed | 800 | 800 | 0 | 0 |
| Validation | Degraded | 8,000 | 7,986 | 14 | 13 |
| Endurance | Degraded | 136,000 | 134,783 | 1,217 | 1,208 |

The heaviest degradation is concentrated in concurrent TCP traffic. The endurance direct path also experiences timeouts, showing that upstream or network behavior contributes to the result.

---

## 13. Future Comparison Design

Baseline JSON files remain immutable source artifacts.

Future comparison tooling should:

- accept an old and new report set;
- verify compatible profile and workload settings;
- compare latency, throughput, reliability, CPU, RSS, temperature, and stability;
- produce generated Markdown or machine-readable comparison output;
- avoid replacing the original reports.

A handwritten optimization log is not required. Generated comparisons should become the performance history.

---

## 14. Known Limitations

Current benchmark limitations include:

- only `A` and `AAAA` query types;
- one upstream target;
- no controlled remote upstream during Raspberry Pi production runs;
- no health snapshot integrated directly into the benchmark report;
- service readiness before and after remains unset;
- no automatic memory-growth interpretation;
- no built-in multi-version comparison command;
- system load may remain at zero during very short runs due to Linux load-average behavior.

These limitations do not invalidate the current baseline.
