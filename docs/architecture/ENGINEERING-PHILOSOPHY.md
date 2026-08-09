# HomeDNS Engineering Philosophy

> **Document type:** Project-wide architecture guidance  
> **Scope:** All HomeDNS components and releases  
> **Status:** Proposed for adoption with Sprint 03  
> **Owner:** HomeDNS Analytics project  
> **Last updated:** August 2026

---

## Document Purpose

This document defines the engineering principles that guide the design, implementation, testing, operation, and evolution of HomeDNS.

It is intentionally independent from any individual sprint. Sprint specifications describe what a release must deliver. Architecture Decision Records explain why a significant technical choice was made. This document defines the principles used to evaluate those choices.

When several implementations satisfy the functional requirements, the implementation most consistent with these principles should be preferred.

These principles are not a substitute for engineering judgment. They provide a common decision framework so that HomeDNS remains coherent as its capabilities grow.

---

## How This Document Is Used

The Engineering Philosophy should be consulted when:

- designing a new subsystem;
- reviewing a sprint specification;
- selecting between competing implementations;
- introducing a new dependency;
- defining a runtime lifecycle;
- evaluating a pull request;
- deciding whether an optimization is justified;
- creating or revising an ADR;
- determining whether technical debt is acceptable.

A design does not need to satisfy every principle equally. When principles conflict, the trade-off must be explicit, measurable where possible, and documented when it affects long-term architecture.

---

# 1. Deterministic Behavior

HomeDNS favors deterministic behavior over implicit or timing-dependent behavior.

Given identical configuration, runtime state, and input, HomeDNS should produce the same observable result.

Runtime behavior should not unexpectedly depend on:

- goroutine scheduling;
- request ordering;
- previous unrelated requests;
- hidden mutable state;
- non-deterministic iteration;
- unseeded randomness;
- filesystem state that was not validated;
- timing races between components.

Determinism makes the system easier to:

- test;
- benchmark;
- reproduce;
- debug;
- reason about;
- operate safely.

Where non-determinism is required, such as randomized identifiers or future load distribution, it should be isolated behind an explicit interface and made controllable in tests.

### Design question

> Can the same input produce a different result for a reason that is not part of the documented contract?

If yes, the source of variation should be removed or made explicit.

---

# 2. Immutable Runtime State

HomeDNS favors immutable runtime state whenever practical.

Configuration and static data should be parsed, validated, normalized, and assembled before the service becomes ready. Request processing should consume stable snapshots rather than mutate shared structures.

Immutable runtime state provides:

- simpler concurrency;
- fewer locks;
- fewer race conditions;
- predictable request behavior;
- reproducible benchmarks;
- easier rollback;
- clearer lifecycle boundaries.

When runtime updates become necessary, HomeDNS should prefer:

1. constructing a complete replacement state;
2. validating it independently;
3. publishing it atomically;
4. allowing existing requests to finish with their original snapshot;
5. retiring the old snapshot safely.

Active shared state should not be modified incrementally unless a documented requirement makes snapshot replacement impractical.

### Design question

> Can this feature publish a new validated snapshot instead of mutating active shared state?

If yes, snapshot replacement should be preferred.

---

# 3. Pipeline-Oriented Architecture

Sequential processing should be represented as explicit, composable stages rather than monolithic orchestration functions.

A pipeline makes:

- execution order visible;
- responsibilities clear;
- intermediate state inspectable;
- error propagation deliberate;
- testing more focused;
- future stages easier to add.

HomeDNS currently applies this principle to several areas.

## Startup lifecycle

```text
Configuration
    -> Validation
    -> Dependency construction
    -> Static data loading
    -> Runtime assembly
    -> Listener activation
    -> Readiness
```

## DNS request processing

```text
Protocol validation
    -> Query construction
    -> Policy evaluation
    -> Future cache lookup
    -> Upstream forwarding or local response
    -> Response writing
    -> Metrics and logging
```

## Policy loading

```text
Source opening
    -> Line parsing
    -> Domain normalization
    -> Validation
    -> Deduplication
    -> Immutable engine construction
```

## Benchmark execution

```text
Profile
    -> Scenario
    -> Runner
    -> Resource collection
    -> Analysis
    -> Report
```

Pipeline architecture does not require a generic framework. A small sequence of explicit interfaces is preferable to an abstract pipeline engine when the abstraction provides no present value.

A stage should exist because it owns a meaningful responsibility, not merely to create more types.

### Design question

> Is this sequential behavior understandable as a small set of independently testable stages?

If yes, the stages should be explicit.

---

# 4. Single Responsibility

Every component should have one primary reason to change.

Examples include:

- the domain normalizer normalizes and validates domain names;
- the rule loader reads and interprets rule sources;
- the policy engine evaluates policy;
- the upstream client performs external DNS resolution;
- the response builder creates DNS responses;
- the metrics registry records measurements;
- the health service reports operational state;
- the benchmark analyzer interprets benchmark results.

A component may coordinate several collaborators, but it should not absorb their responsibilities.

Signs that a component may have too many responsibilities include:

- unrelated configuration fields;
- extensive branching for different concerns;
- tests that require many unrelated fixtures;
- changes in one feature frequently breaking another;
- infrastructure dependencies inside domain logic;
- a type that must understand policy, DNS transport, metrics, and persistence simultaneously.

### Design question

> What is the single sentence that explains why this component exists?

If that sentence requires several unrelated clauses, the component should be reconsidered.

---

# 5. Composition Over Complexity

HomeDNS favors small cooperating components over large feature-rich objects.

Composition should be used to assemble behavior from clear responsibilities. It should not be used to create unnecessary indirection.

The preferred progression is:

1. implement the simplest correct component;
2. introduce an interface when there is a real boundary, test seam, or alternative implementation;
3. compose components at the application boundary;
4. keep domain logic independent from infrastructure details.

Abstractions should solve a current problem. They should not attempt to predict every future policy type, storage engine, transport, dashboard, or deployment target.

### Design question

> Does this abstraction simplify a current responsibility, or does it only prepare for an imagined future?

Speculative abstraction should be deferred.

---

# 6. Explicit Domain Models

Business concepts should be represented through explicit types rather than primitive values with hidden meanings.

Examples include:

- `Query` instead of passing a raw domain string through the policy layer;
- `Decision` instead of returning a boolean;
- `Action` instead of an arbitrary string;
- `Reason` instead of inferring intent from a counter name;
- `Rule` instead of storing only unstructured file lines;
- `RuleSource` instead of passing anonymous paths;
- structured domain errors instead of generic formatting errors;
- policy loading statistics instead of scattered counters.

Explicit models improve:

- readability;
- compiler assistance;
- validation;
- discoverability;
- logging consistency;
- test precision;
- future compatibility.

Types should model real business distinctions. Creating wrapper types without meaningful invariants or behavior is not required.

### Design question

> Is business meaning currently encoded in a boolean, arbitrary string, integer, or positional parameter?

If yes, an explicit type may be more appropriate.

---

# 7. Startup Validation Over Runtime Surprises

HomeDNS should discover invalid static state before becoming ready.

Startup is the correct place to validate:

- configuration syntax;
- unknown fields;
- required relationships between fields;
- file existence and readability;
- file type and safety constraints;
- rule formats;
- dependency construction;
- listener availability;
- immutable snapshot construction.

Readiness should mean that the service is capable of performing its documented function with the accepted configuration.

A service should not report ready while depending on static data that it has not successfully loaded or validated.

This principle does not mean every external dependency must remain continuously available. Runtime failures are handled according to the documented availability strategy for that dependency.

### Design question

> Can this failure be detected before the service accepts traffic?

If yes, it should normally fail startup rather than surprise runtime processing.

---

# 8. Availability With Explicit Failure Boundaries

HomeDNS protects availability without silently weakening correctness.

The general model is:

- startup favors correctness;
- runtime favors continued DNS availability where a safe fallback exists;
- invalid client input is rejected;
- explicit policy decisions are respected;
- runtime fallback behavior is observable.

For example, an enabled policy that cannot be constructed must prevent startup. After successful startup, an unexpected policy evaluation error may fail open for that request because DNS availability is essential and the immutable policy was already validated.

Failure behavior must be deliberate. Components should not independently invent fallback behavior.

Every fallback should define:

- the boundary where it applies;
- the conditions that trigger it;
- the response or alternative path;
- the metrics recorded;
- the logs emitted;
- the scenarios where it is prohibited.

### Design question

> What exactly happens when this component fails, and where is that behavior defined?

If the answer is implicit, the design is incomplete.

---

# 9. Observability by Design

Important behavior should be observable without changing business decisions.

HomeDNS observability includes:

- health and readiness;
- aggregate runtime metrics;
- structured logs;
- benchmark reports;
- startup summaries;
- resource measurements;
- release validation evidence.

Observability remains passive. A metrics backend, log sink, or dashboard should not determine whether a DNS request is allowed, blocked, forwarded, or answered.

Observability should be designed with the feature rather than added after implementation. Every important path should answer:

- How do we know it is working?
- How do we know it is slow?
- How do we know it is failing?
- How do we know behavior changed after a release?

High-cardinality and sensitive dimensions should be avoided. Domain names, source paths, client addresses, and rule contents should not become unbounded metric labels.

### Design question

> Could an operator distinguish healthy, degraded, and failing behavior using the available signals?

If not, the feature is not operationally complete.

---

# 10. Benchmark-Driven Engineering

Performance decisions should be based on repeatable measurements rather than intuition.

HomeDNS runs on constrained hardware and participates in a latency-sensitive network path. Performance is therefore part of correctness.

Benchmarking should measure the properties relevant to the feature, including where applicable:

- latency;
- throughput;
- correctness;
- memory usage;
- CPU usage;
- goroutine or thread count;
- system memory headroom;
- temperature;
- throttling;
- startup duration;
- loading duration;
- external dependency calls.

Benchmark changes should preserve historical comparability whenever possible. Existing profiles and report semantics should not be rewritten merely to add a new feature.

An optimization should normally follow this sequence:

1. define the suspected problem;
2. capture a baseline;
3. change one relevant factor;
4. rerun the same workload;
5. compare correctness and resource use;
6. retain the change only when the result justifies its complexity.

### Design question

> What measurement demonstrates that this design is acceptable or better?

If no meaningful measurement exists, performance claims should remain hypotheses.

---

# 11. Testability as an Architectural Requirement

Testability is a property of the design, not only of the test suite.

Components should support focused verification through:

- table-driven unit tests;
- integration tests;
- protocol-level tests;
- deterministic fakes;
- reusable test fixtures;
- fuzzing;
- race detection;
- controlled failure injection;
- target-hardware benchmarks.

A component that can only be tested by starting the entire application may have unclear boundaries or excessive coupling.

Tests should verify observable behavior and contracts rather than private storage layout. Internal implementation may evolve while domain behavior remains stable.

Production paths should not contain special behavior solely for tests. Test seams should come from normal dependency boundaries.

### Design question

> Can this component be tested deterministically without reproducing the whole production environment?

If not, its dependencies and responsibility should be reviewed.

---

# 12. Backward Compatibility

HomeDNS preserves compatibility whenever doing so does not compromise safety or create disproportionate complexity.

Compatibility includes:

- existing configuration files;
- release and rollback procedures;
- deployment directory conventions;
- systemd behavior;
- health contracts;
- metric meanings;
- benchmark report schemas;
- operator workflows.

New optional features should normally default to preserving existing behavior.

Breaking changes may be appropriate, but they must be:

- deliberate;
- documented;
- justified;
- migration-aware;
- validated through tests and deployment procedures.

Compatibility does not require retaining accidental behavior, undocumented bugs, or unsafe configuration.

### Design question

> What existing deployment, tool, report, or operator workflow could this change break?

The answer should be known before merge.

---

# 13. Security and Privacy by Default

HomeDNS should minimize privileges, exposed information, and mutable attack surface.

Security principles include:

- run with the least required filesystem and network permissions;
- treat DNS requests as untrusted input;
- validate static operator-controlled input before readiness;
- avoid runtime writes where they are unnecessary;
- keep health and metrics aggregate;
- avoid sensitive or high-cardinality metric labels;
- keep individual allowed domains out of routine logs;
- log blocked domains only at an explicitly selected diagnostic level;
- avoid exposing internal paths, rule contents, or implementation details;
- bound input sizes and warning volume;
- isolate malformed input rather than allowing it to corrupt valid state.

A debugging convenience should not become an accidental permanent data-collection feature.

### Design question

> Does this feature expose or retain more information or privilege than its function requires?

If yes, the design should be reduced.

---

# 14. Resource Efficiency on the Reference Hardware

HomeDNS is designed for the Raspberry Pi reference platform, not only for developer workstations.

Design decisions should account for:

- approximately 1 GB of memory;
- limited CPU capacity;
- SD-card longevity;
- thermal constraints;
- shared operation with other lightweight services;
- continuous availability;
- DNS latency sensitivity.

Resource efficiency does not mean prematurely minimizing every allocation. It means validating that architecture and workloads remain appropriate for the target hardware.

The desktop development environment is useful for fast feedback, but it cannot replace Raspberry Pi validation for release acceptance.

### Design question

> Is this design still appropriate when memory, CPU, storage, and thermal headroom are constrained?

If the answer is unknown, it must be measured on the reference device.

---

# 15. Documentation as Part of the Product

Documentation is a delivered project capability, not an afterthought.

Relevant changes should update the appropriate combination of:

- sprint specifications;
- architecture documents;
- ADRs;
- configuration references;
- operations runbooks;
- benchmark methodology;
- user guidance;
- release notes;
- troubleshooting procedures.

Documentation should describe implemented behavior. Aspirational or deferred behavior must be clearly identified as such.

Outdated documentation is considered a defect because it creates incorrect operational and architectural assumptions.

### Design question

> Could a contributor or operator understand and safely use this change from the repository documentation?

If not, the change is incomplete.

---

# 16. Incremental Delivery

HomeDNS favors small, independently reviewable changes over large integration branches.

Each implementation increment should:

- have one coherent objective;
- compile successfully;
- pass applicable tests;
- preserve a usable `develop` branch;
- include documentation when behavior or architecture changes;
- avoid unrelated cleanup;
- be reversible or easy to isolate.

Large features should be decomposed into domain foundations, infrastructure components, integration changes, observability, benchmarks, and documentation where appropriate.

Incremental delivery reduces:

- review complexity;
- merge conflicts;
- hidden regressions;
- architectural drift;
- rollback difficulty.

### Design question

> Can this change be split into a smaller independently useful and verifiable increment?

If yes, the smaller increment should be preferred.

---

# 17. Simplicity Before Premature Generalization

HomeDNS should solve the current problem cleanly before generalizing for possible future problems.

The project should avoid:

- plugin systems without multiple real plugins;
- generic repositories without multiple persistence needs;
- universal pipeline frameworks for a handful of known stages;
- configuration options without an immediate supported behavior;
- abstractions that make basic control flow difficult to follow;
- dependency-heavy solutions for problems that can be solved clearly in the standard library.

Simplicity does not mean placing everything in one function. It means using the minimum architecture required to preserve correctness, clarity, testability, and future extension.

### Design question

> Is this complexity required by a current accepted requirement?

If not, it should usually be deferred.

---

# 18. Explicit Dependency Direction

Domain logic should not depend on transport, filesystem, process supervision, monitoring backends, or presentation concerns.

Dependencies should generally point inward:

```text
Application assembly
    -> Infrastructure adapters
    -> Application interfaces and orchestration
    -> Domain behavior and models
```

Examples:

- the policy engine evaluates a HomeDNS `Query`, not a `dns.Msg`;
- the loader produces domain rules but the engine does not read files;
- the upstream adapter understands DNS transport but not policy precedence;
- the health handler reads a health view but does not own policy state;
- benchmark tooling observes the service but does not alter production behavior.

Interfaces should be defined near the consumer when practical, keeping consumers independent from concrete infrastructure.

### Design question

> Is a business component importing an infrastructure detail that it does not need to understand?

If yes, the dependency boundary should be reconsidered.

---

# 19. Measured Evolution Over Rewrites

HomeDNS should extend proven foundations unless evidence shows that they no longer satisfy requirements.

A new sprint should not rewrite a subsystem simply because a different architecture is attractive. Refactoring should respond to a concrete limitation such as:

- incorrect behavior;
- untestable coupling;
- measured performance problems;
- operational risk;
- repeated implementation friction;
- inability to support an approved requirement.

Where a subsystem already has suitable boundaries, new behavior should plug into those boundaries. The Sprint 03 policy benchmarks, for example, extend the existing Sprint 02 benchmark system instead of replacing it.

### Design question

> What demonstrated limitation requires this rewrite?

Without a clear answer, extension is preferred over replacement.

---

# 20. Long-Term Maintainability

HomeDNS should remain understandable to a contributor who did not participate in its original design.

Maintainability requires:

- clear package boundaries;
- predictable naming;
- explicit contracts;
- limited hidden behavior;
- documented architectural decisions;
- focused tests;
- restrained dependency use;
- consistent operational procedures;
- code that favors readability over cleverness.

Short-term convenience should not introduce permanent ambiguity. At the same time, maintainability should not be used to justify speculative frameworks.

The desired outcome is software that is simple to understand because its responsibilities and decisions are explicit.

### Design question

> Could a new contributor understand why this code exists and how it behaves without relying on oral history?

If not, the implementation or documentation should be improved.

---

# Applying the Philosophy

## During specification

A sprint specification should identify:

- the domain capability being introduced;
- the lifecycle of its state;
- its pipeline or orchestration boundary;
- startup and runtime failure behavior;
- observability requirements;
- testing and benchmark strategy;
- compatibility impact;
- explicitly deferred work.

## During implementation

A contributor should verify that:

- responsibilities remain focused;
- domain behavior is independent from infrastructure;
- shared runtime state is immutable or intentionally synchronized;
- error behavior is explicit;
- tests cover contracts and edge cases;
- metrics do not influence decisions;
- resource costs remain appropriate for the Raspberry Pi;
- documentation evolves with the code.

## During pull-request review

Reviewers should ask:

1. Is the behavior deterministic?
2. Is shared state immutable where practical?
3. Are responsibilities and dependency directions clear?
4. Is sequential processing explicit?
5. Is failure behavior defined at the correct boundary?
6. Can the change be tested without excessive environment setup?
7. Is observability sufficient and privacy-conscious?
8. Is performance measured where it matters?
9. Does the change preserve compatibility or document its migration?
10. Is the complexity justified by an accepted requirement?
11. Are documentation and operational procedures current?
12. Has the change been validated on the reference hardware when required?

---

# Exceptions and Trade-offs

These principles guide decisions; they do not prohibit justified exceptions.

An exception should be documented when it introduces a lasting architectural difference or weakens a project-wide invariant.

The decision record should explain:

- the requirement creating the conflict;
- the principles affected;
- alternatives considered;
- evidence supporting the chosen trade-off;
- risks and mitigations;
- whether the exception is temporary or permanent;
- how the decision will be revisited.

A local implementation detail does not require an ADR merely because it differs from a preference. An ADR is appropriate when the decision affects multiple components, future releases, operational behavior, compatibility, or a core invariant.

---

# Relationship to Other Documents

## Roadmap

`ROADMAP.md` defines the sequence of product capabilities and milestones.

The Engineering Philosophy guides how those capabilities should be designed and delivered.

## Sprint specifications

Documents under `docs/sprints/` define the normative scope, behavior, architecture, tests, benchmarks, and acceptance criteria of a specific sprint.

Every future sprint specification should reference this document rather than duplicate the full philosophy.

## Architecture Decision Records

ADRs capture significant decisions that apply these principles to a concrete architectural choice.

Examples that should be formalized or retained include:

- clean architecture and dependency direction;
- benchmark-first development;
- immutable runtime policy snapshots;
- pipeline-oriented request processing;
- runtime policy fail-open behavior;
- the local NXDOMAIN blocking strategy.

Exact ADR numbering should follow the repository state when each record is created.

## Operations documentation

Operations documents explain how to deploy, configure, monitor, troubleshoot, roll back, and validate HomeDNS.

Operational simplicity, explicit readiness, bounded resource use, and deterministic startup originate from this philosophy.

---

# Principle Summary

HomeDNS favors:

- deterministic behavior over hidden state;
- immutable snapshots over active shared mutation;
- explicit pipelines over monolithic processing;
- single-purpose components over mixed responsibilities;
- composition over speculative frameworks;
- explicit domain models over primitive meanings;
- startup validation over runtime surprises;
- deliberate failure boundaries over accidental fallback;
- passive observability over monitoring-driven behavior;
- measured performance over assumptions;
- testable architecture over environment-heavy verification;
- backward compatibility over unnecessary disruption;
- security and privacy by default;
- Raspberry Pi validation over workstation-only confidence;
- documentation as part of the product;
- incremental delivery over large integration branches;
- simplicity over premature generalization;
- clean dependency direction over infrastructure leakage;
- measured evolution over rewrites;
- long-term clarity over short-term cleverness.

---

# Closing Statement

HomeDNS is intended to grow through small, measurable, well-documented architectural increments.

Its engineering identity is defined not by the number of features it contains, but by the predictability, clarity, efficiency, and operational confidence with which those features are delivered.

These principles provide the common foundation for that evolution.
