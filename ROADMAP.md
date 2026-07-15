# HomeDNS Analytics — Project Roadmap

> A lightweight, self-hosted DNS filtering and network analytics platform designed to run 24/7 on a Raspberry Pi 3B.

---

## 1. Project Vision

HomeDNS Analytics is a headless DNS filtering and observability platform for home networks.

It will:

- resolve and forward DNS queries;
- block advertising, tracking, and malicious domains;
- support custom blacklists and whitelists;
- collect DNS usage statistics;
- provide historical and per-device analytics;
- expose a REST API;
- provide a responsive web dashboard accessible from another device;
- run reliably on a Raspberry Pi 3B;
- be administered through SSH;
- use automated testing, packaging, and deployment practices.

The project is intended to be both a useful home-network appliance and a production-quality portfolio project.

---

## 2. Primary Objectives

### Functional objectives

- Serve DNS requests over UDP and TCP.
- Forward allowed requests to configurable upstream DNS resolvers.
- Block domains using local and downloaded rules.
- Allow users to override downloaded rules with a whitelist.
- Cache DNS responses while respecting TTL values.
- record DNS activity without delaying DNS responses.
- Store raw query history and aggregated statistics.
- Display network activity through a React dashboard.
- Manage clients, lists, rules, and settings through a FastAPI API.
- Export query data and statistics.

### Technical objectives

- Keep memory and CPU usage compatible with a Raspberry Pi 3B.
- Keep DNS resolution operational even when the dashboard is unavailable.
- Avoid a permanent Node.js process on the Raspberry Pi.
- Avoid Docker in the first production version.
- Use SQLite as the embedded database.
- Use systemd to supervise services.
- Use GitHub Actions for continuous integration and release packaging.
- Support safe SSH deployment, health checks, and rollback.

### Quality objectives

- Automated unit, integration, and API tests.
- Reproducible builds.
- Versioned database migrations.
- Structured logs.
- Documented installation and recovery procedures.
- No secrets committed to the repository.
- Measured resource usage and DNS latency.

---

## 3. Target Environment

### Hardware

- Raspberry Pi 3 Model B
- 1 GB RAM
- Ethernet connection recommended
- High-quality microSD card or external USB storage
- Stable power supply

### Operating system

- Raspberry Pi OS Lite 64-bit
- No desktop environment
- SSH administration only
- Static DHCP lease or fixed local IP address

### Expected usage

Initial target:

- 5–30 network clients
- 20,000–100,000 DNS queries per day
- 200,000–500,000 unique blocked domains
- 7–30 days of raw query retention
- Long-term hourly and daily aggregates

---

## 4. Technology Stack

### DNS service

- **Language:** Go
- **DNS library:** `github.com/miekg/dns`
- **Responsibilities:**
  - UDP and TCP DNS servers
  - upstream forwarding
  - blocking and allowlisting
  - in-memory rule matching
  - DNS cache
  - asynchronous query logging
  - runtime health and metrics
  - rule reload and cache management

### Management API

- **Language:** Python
- **Framework:** FastAPI
- **Server:** Uvicorn, one worker
- **Database layer:** SQLAlchemy
- **Migrations:** Alembic
- **Responsibilities:**
  - dashboard API
  - query explorer
  - rule management
  - blocklist source management
  - client management
  - settings
  - authentication
  - analytics queries
  - exports
  - communication with the DNS service

### Frontend

- **Framework:** React
- **Build tool:** Vite
- **Data fetching:** TanStack Query
- **Routing:** React Router
- **Charts:** Recharts
- **Testing:** Vitest and React Testing Library
- **Deployment:** static production build served by FastAPI or a lightweight reverse proxy

Node.js will only be required during development and CI. It will not run permanently on the Raspberry Pi.

### Database

- **Database:** SQLite
- **Mode:** WAL
- **Responsibilities:**
  - raw query logs
  - hourly and daily aggregates
  - custom blacklist and whitelist rules
  - external blocklist sources
  - clients
  - settings
  - update history
  - application users

### Operations

- Git and GitHub
- GitHub Actions
- systemd
- SSH
- Makefile
- shell deployment scripts
- Ruff and pytest
- Go test and golangci-lint
- ESLint and Vitest

---

## 5. High-Level Architecture

```text
        Network clients
              |
              | DNS UDP/TCP :53
              v
+---------------------------+
| Go DNS Service            |
|                           |
| - Rule matcher            |
| - DNS cache               |
| - Upstream forwarder      |
| - Query event buffer      |
| - Internal health API     |
+-------------+-------------+
              |
              | batched writes
              v
+---------------------------+
| SQLite                    |
|                           |
| - Query history           |
| - Aggregated statistics   |
| - Rules and list sources  |
| - Clients and settings    |
+-------------+-------------+
              ^
              |
+-------------+-------------+
| FastAPI Management API    |
|                           |
| - Analytics endpoints     |
| - Configuration endpoints |
| - Authentication          |
| - Static React files      |
+-------------+-------------+
              |
              | HTTP from LAN
              v
+---------------------------+
| Browser on laptop/phone   |
| React dashboard           |
+---------------------------+
```

### Service isolation

The DNS service and management API will run as separate systemd services.

A dashboard failure must not stop DNS resolution.

```text
homedns-dns.service
homedns-api.service
```

---

## 6. Repository Structure

```text
homedns/
├── .github/
│   └── workflows/
│       ├── ci.yml
│       ├── release.yml
│       └── deploy.yml
│
├── dns/
│   ├── cmd/
│   │   └── server/
│   ├── internal/
│   │   ├── blocking/
│   │   ├── cache/
│   │   ├── config/
│   │   ├── database/
│   │   ├── dnsserver/
│   │   ├── events/
│   │   ├── health/
│   │   ├── metrics/
│   │   └── upstream/
│   ├── tests/
│   ├── go.mod
│   └── go.sum
│
├── backend/
│   ├── app/
│   │   ├── api/
│   │   ├── core/
│   │   ├── models/
│   │   ├── repositories/
│   │   ├── schemas/
│   │   └── services/
│   ├── migrations/
│   ├── tests/
│   └── pyproject.toml
│
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── features/
│   │   ├── pages/
│   │   └── types/
│   ├── tests/
│   └── package.json
│
├── deploy/
│   ├── scripts/
│   ├── systemd/
│   └── config/
│
├── docs/
│   ├── architecture/
│   ├── decisions/
│   ├── operations/
│   └── sprints/
│
├── scripts/
├── dist/
├── Makefile
├── ROADMAP.md
├── CONTRIBUTING.md
└── README.md
```

---

## 7. Core Data Model

The detailed schema will be designed during the persistence sprint.

Initial entities:

### Query log

- timestamp
- client IP
- requested domain
- DNS query type
- result status
- blocked status
- blocking source
- response code
- upstream resolver
- response latency
- cache hit status

### Client

- IP address
- hostname or display name
- first seen timestamp
- last seen timestamp
- enabled status
- optional profile

### Domain rule

- domain or pattern
- rule type: allow or block
- enabled status
- source
- comment
- creation timestamp

### Blocklist source

- name
- source URL
- enabled status
- last update timestamp
- update status
- downloaded domain count
- unique domain count

### Aggregated statistics

- time bucket
- client
- total queries
- blocked queries
- cached queries
- upstream queries
- NXDOMAIN responses
- SERVFAIL responses
- average latency

### Settings

- upstream resolvers
- retention duration
- batch size
- cache limits
- list update schedule
- interface configuration

---

## 8. DNS Request Processing Order

The initial request flow will be:

```text
1. Receive and validate DNS request
2. Normalize requested domain
3. Check explicit whitelist
4. Check explicit user blacklist
5. Check downloaded blocklists
6. Check in-memory DNS cache
7. Forward request to an upstream resolver
8. Cache a valid response using its TTL
9. Return the response to the client
10. Publish a query event to the asynchronous logger
```

The whitelist has priority over downloaded blocklists.

Database access must not occur synchronously in the critical DNS response path.

---

## 9. Non-Functional Requirements

### Performance

Initial targets:

- DNS service idle CPU close to 0%.
- Typical DNS processing overhead below 5 ms, excluding upstream latency.
- DNS response logging must not block query responses.
- API must remain responsive with 30 days of retained raw data.
- Active application memory target below 550 MB for the complete system.
- No swap usage during normal operation.

These values are project targets and must be verified through benchmarks.

### Reliability

- Services start automatically after reboot.
- Services restart after unexpected failure.
- DNS remains operational if FastAPI or React fails.
- A bounded event queue prevents unlimited memory growth.
- Failed database logging does not stop DNS resolution.
- Configuration is validated before service startup.
- Deployment includes health checks.
- Deployment can roll back to the previous release.

### Security

- SSH key authentication.
- No public exposure of the management interface by default.
- API bound to the local network or localhost behind a proxy.
- Passwords stored using a strong password hashing algorithm.
- Secrets loaded from protected environment or configuration files.
- Internal DNS service management endpoints restricted to localhost.
- Dependency and secret scanning in CI.
- systemd service hardening.
- Principle of least privilege.

### Storage

- SQLite WAL mode.
- Batched query inserts.
- Indexed analytics fields.
- Configurable raw-data retention.
- Hourly and daily aggregation.
- Regular database backups.
- Log rotation.
- Database size monitoring.

---

# 10. Delivery Roadmap

Each sprint will receive its own detailed document under:

```text
docs/sprints/
```

Suggested naming convention:

```text
SPRINT-00-PROJECT-FOUNDATION.md
SPRINT-01-RASPBERRY-INFRASTRUCTURE.md
SPRINT-02-DNS-FORWARDER.md
...
```

---

## Sprint 0 — Project Foundation

### Goal

Create the repository, engineering conventions, documentation structure, and local development workflow.

### Main tasks

- Select the final project name.
- Create the GitHub repository.
- Add the initial directory structure.
- Create `README.md`, `ROADMAP.md`, and `CONTRIBUTING.md`.
- Define branch and commit conventions.
- Add issue and pull-request templates.
- Add a root `Makefile`.
- Add editor and formatting configuration.
- Add initial architecture decision records.
- Define versioning and release conventions.
- Create an initial GitHub Projects board or issue backlog.

### Deliverables

- Initialized repository.
- Documented project vision and stack.
- Reproducible local commands.
- Initial backlog.
- First architectural decisions recorded.

### Exit criteria

- Repository can be cloned and initialized.
- Every component has a placeholder project.
- `make help` documents available commands.
- CI can run a basic placeholder workflow.

---

## Sprint 1 — Raspberry Pi Infrastructure

### Goal

Prepare a secure, stable, headless Raspberry Pi environment.

### Main tasks

- Install Raspberry Pi OS Lite 64-bit.
- Enable SSH.
- Configure SSH key authentication.
- Disable password authentication after validation.
- Assign a static DHCP lease or fixed IP.
- Configure hostname and local DNS name.
- Update system packages.
- Create a dedicated `homedns` system user.
- Create application directories under `/opt/homedns`.
- Configure time synchronization and timezone.
- Install required runtime packages.
- Configure firewall rules.
- Add basic monitoring commands and scripts.
- Document backup and recovery access.

### Deliverables

- Reachable headless Raspberry Pi.
- Secure SSH configuration.
- Dedicated application user and directories.
- Infrastructure setup documentation.

### Exit criteria

- Pi can reboot and remain reachable.
- SSH works using keys.
- Application user cannot obtain unnecessary privileges.
- Required ports are documented and controlled.
- Baseline RAM, CPU, temperature, and storage usage are recorded.

---

## Sprint 2 — DNS Forwarder MVP

### Goal

Build a functional Go DNS server that forwards DNS requests to an upstream resolver.

### Main tasks

- Initialize the Go module.
- Define configuration structures.
- Start UDP and TCP DNS listeners.
- Parse DNS requests using `miekg/dns`.
- Forward requests to one configurable upstream resolver.
- Return upstream responses to clients.
- Add request timeout handling.
- Add structured logging.
- Handle common response errors.
- Support graceful shutdown.
- Add a basic internal health endpoint.
- Add unit and integration tests.

### Deliverables

- Working DNS forwarder.
- UDP and TCP support.
- Configurable upstream resolver.
- Health endpoint.
- Automated tests.

### Exit criteria

- `dig` can resolve A, AAAA, and CNAME queries through the service.
- TCP fallback works.
- Invalid requests do not crash the server.
- Upstream timeout returns an appropriate DNS error.
- Service shuts down cleanly.
- DNS forwarding tests pass in CI.

---

## Sprint 3 — Filtering Engine

### Goal

Add blacklist, whitelist, and downloaded blocklist filtering.

### Main tasks

- Define normalized domain-rule formats.
- Implement exact-domain matching.
- Implement parent-domain or suffix matching.
- Add explicit whitelist priority.
- Add user blacklist support.
- Parse common hosts-file blocklist formats.
- Merge and deduplicate downloaded lists.
- Load active rules into memory.
- Reload rules without restarting the DNS service.
- Return configurable blocked responses.
- Record the rule source responsible for blocking.
- Add filtering unit tests and benchmarks.

### Deliverables

- In-memory blocking engine.
- Whitelist override behavior.
- Blocklist parser.
- Runtime reload endpoint.
- Rule-matching test suite.

### Exit criteria

- Exact and subdomain rules behave as documented.
- Whitelist rules override downloaded blocklists.
- Rule reload does not interrupt DNS traffic.
- Large test blocklists load within an acceptable duration.
- Filtering lookup benchmarks meet the initial target.

---

## Sprint 4 — DNS Cache

### Goal

Reduce upstream traffic and latency using a bounded in-memory DNS cache.

### Main tasks

- Define the cache key.
- Store successful DNS responses.
- Respect record TTL values.
- Expire stale entries.
- Add negative caching where appropriate.
- Add maximum entry and memory limits.
- Define an eviction policy.
- Track hit, miss, and eviction metrics.
- Add cache flush and inspection endpoints.
- Add concurrency and race-condition tests.
- Benchmark cached and uncached responses.

### Deliverables

- Thread-safe bounded DNS cache.
- Cache metrics.
- Cache management endpoints.
- Cache test and benchmark suite.

### Exit criteria

- Cached responses preserve valid TTL behavior.
- Expired responses are never returned.
- Cache memory remains bounded.
- Concurrent access passes race detection.
- Cache hit responses are measurably faster than upstream queries.

---

## Sprint 5 — SQLite Persistence

### Goal

Persist queries, clients, rules, settings, and statistics without affecting DNS latency.

### Main tasks

- Design the initial SQLite schema.
- Enable WAL mode.
- Create versioned migrations.
- Implement a bounded query-event channel in Go.
- Implement timed and size-based batch inserts.
- Handle temporary database failures.
- Add client discovery and last-seen updates.
- Store custom rules and list sources.
- Add database indexes.
- Implement raw query retention.
- Implement hourly and daily aggregation.
- Add database backup scripts.
- Test concurrent API reads and DNS writes.

### Deliverables

- Versioned SQLite schema.
- Buffered event pipeline.
- Retention and aggregation jobs.
- Backup procedure.
- Database tests.

### Exit criteria

- DNS responses do not wait for database inserts.
- Query events are written in batches.
- Dashboard-style read queries work during writes.
- Database recovers correctly after restart.
- Retention removes expired raw records.
- Aggregates remain available after raw records are deleted.

---

## Sprint 6 — FastAPI Management API

### Goal

Expose configuration, query, and analytics operations through a documented API.

### Main tasks

- Initialize FastAPI and SQLAlchemy.
- Configure Alembic integration.
- Add application settings.
- Add database repositories and services.
- Add health endpoint.
- Add overview statistics endpoint.
- Add time-series statistics endpoint.
- Add query explorer with pagination and filters.
- Add client endpoints.
- Add blacklist and whitelist CRUD endpoints.
- Add blocklist source CRUD and update endpoints.
- Add settings endpoints.
- Add DNS service control client.
- Add validation and error handling.
- Generate OpenAPI documentation.
- Add API tests.

### Deliverables

- Versioned REST API.
- OpenAPI documentation.
- Repository and service layers.
- API test suite.
- DNS service integration.

### Exit criteria

- Required dashboard data is available through the API.
- Query filtering and pagination work.
- Rule changes can trigger safe DNS rule reloads.
- Errors use a consistent response format.
- API tests pass in CI.
- FastAPI failure does not affect DNS resolution.

---

## Sprint 7 — Authentication and Security

### Goal

Protect the management interface and harden service communication.

### Main tasks

- Add a local administrator account.
- Implement secure password hashing.
- Implement login and logout.
- Add short-lived access tokens or secure sessions.
- Protect management endpoints.
- Add rate limiting for authentication.
- Restrict internal DNS control endpoints to localhost.
- Validate CORS and host settings.
- Add secure HTTP headers.
- Review file and database permissions.
- Harden systemd units.
- Add security-related tests.
- Document LAN-only and remote-access options.

### Deliverables

- Protected management API.
- Authentication flow.
- Hardened system services.
- Security documentation.

### Exit criteria

- Unauthenticated users cannot access protected API routes.
- Credentials are never stored in plain text.
- Internal DNS endpoints are inaccessible from the LAN.
- Services run without root after binding requirements are addressed.
- Security tests pass.

---

## Sprint 8 — React Dashboard MVP

### Goal

Provide a responsive browser-based interface for monitoring and managing HomeDNS.

### Main tasks

- Initialize React and Vite.
- Define frontend architecture and API client.
- Add routing and application layout.
- Add login page.
- Add dashboard overview cards.
- Add query volume chart.
- Add blocked query chart.
- Add top-domain and top-client tables.
- Add query explorer.
- Add loading, empty, and error states.
- Add responsive navigation.
- Add frontend tests.
- Build static assets in CI.
- Serve the built frontend through FastAPI.

### Deliverables

- Responsive dashboard.
- Overview analytics.
- Query explorer.
- Production static build.
- Frontend test suite.

### Exit criteria

- Dashboard works on desktop and mobile browsers.
- No Node.js server is required in production.
- API errors are displayed clearly.
- Main pages are keyboard accessible.
- Production build is generated by CI.

---

## Sprint 9 — Rules, Lists, Clients, and Settings UI

### Goal

Complete the main administration workflows.

### Main tasks

- Add blacklist management page.
- Add whitelist management page.
- Add external blocklist source page.
- Add list update status and history.
- Add manual list update action.
- Add client list and client detail page.
- Allow client display names.
- Add upstream resolver settings.
- Add retention and cache settings.
- Add confirmation dialogs.
- Add form validation and notifications.
- Add relevant frontend and API tests.

### Deliverables

- Complete rule-management UI.
- Blocklist update UI.
- Client management UI.
- Settings UI.

### Exit criteria

- Users can add, edit, enable, disable, and delete local rules.
- Users can manage external list sources.
- Rule changes become effective without service interruption.
- Client names persist.
- Invalid configuration cannot be saved.

---

## Sprint 10 — Analytics and Data Intelligence

### Goal

Turn raw DNS activity into useful network insights.

### Main tasks

- Add hourly, daily, and weekly trend views.
- Add per-client analytics.
- Add blocklist effectiveness analytics.
- Add first-seen domain detection.
- Add unusual query-rate detection.
- Add NXDOMAIN and SERVFAIL monitoring.
- Add cache-efficiency analytics.
- Add upstream latency comparison.
- Add CSV and JSON export.
- Add scheduled aggregation verification.
- Optimize analytics queries and indexes.

### Deliverables

- Historical analytics pages.
- Per-client insights.
- DNS intelligence indicators.
- Data export.

### Exit criteria

- Historical charts remain responsive with the target data volume.
- Aggregated values match raw-query samples.
- Users can identify top clients, domains, and blocking sources.
- Exported data respects filters.
- Analytics query performance is measured and documented.

---

## Sprint 11 — CI Pipeline

### Goal

Automatically validate every proposed change.

### Main tasks

- Add pull-request and `main` branch triggers.
- Add Go formatting, vetting, linting, tests, and builds.
- Add Python Ruff, formatting, type checking, and pytest.
- Add frontend linting, tests, and production builds.
- Cache dependencies safely.
- Add coverage reports.
- Add dependency scanning.
- Add secret scanning.
- Add migration validation.
- Add ARM64 cross-compilation.
- Protect the `main` branch using required checks.

### Deliverables

- Complete GitHub Actions CI workflow.
- Required pull-request checks.
- Build and test artifacts.
- Documented local CI-equivalent command.

### Exit criteria

- A failing component blocks merge.
- All components are validated in parallel.
- ARM64 DNS binary builds successfully.
- Frontend production assets build successfully.
- CI commands can also run locally through `make ci`.

---

## Sprint 12 — Release Packaging

### Goal

Produce reproducible, versioned release artifacts for the Raspberry Pi.

### Main tasks

- Define semantic versioning.
- Build the Go ARM64 binary.
- Package the FastAPI source and locked dependencies.
- Package the React static build.
- Include migrations and service files.
- Include checksums.
- Generate release notes.
- Publish tagged GitHub releases.
- Validate packages in a clean environment.
- Document upgrade compatibility.

### Deliverables

- Versioned release archive.
- Checksums.
- GitHub release workflow.
- Release notes template.

### Exit criteria

- A release can be installed without cloning the source repository.
- Release artifacts are generated entirely by CI.
- Checksums verify successfully.
- The package contains all required runtime files.
- Version information appears in health endpoints and the dashboard.

---

## Sprint 13 — SSH Deployment and Rollback

### Goal

Deploy releases safely to the Raspberry Pi through SSH.

### Main tasks

- Create versioned release directories.
- Separate shared configuration and data.
- Create install and deployment scripts.
- Back up SQLite before migrations.
- Install backend dependencies into a release-specific virtual environment.
- Run database migrations.
- Atomically switch the `current` symlink.
- Restart systemd services.
- Run API and DNS health checks.
- Roll back automatically on failure.
- Keep a configurable number of previous releases.
- Add manual deployment from the development laptop.

### Deliverables

- SSH deployment script.
- Health-check script.
- Rollback script.
- Versioned deployment layout.
- Deployment runbook.

### Exit criteria

- A release can be deployed with one documented command.
- Failed health checks restore the previous release.
- Database and configuration survive deployment.
- Old releases can be cleaned safely.
- Deployment does not require opening SSH to the public internet.

---

## Sprint 14 — Automated Delivery

### Goal

Automate production deployment while keeping the home network private.

### Main tasks

- Select Tailscale, a self-hosted runner, or another private deployment path.
- Configure a protected GitHub production environment.
- Require manual approval for production.
- Download release artifacts in the deployment job.
- Deploy only tagged releases.
- Run post-deployment health checks.
- Report deployment status.
- Prevent concurrent deployments.
- Document emergency manual deployment.

### Deliverables

- Protected CD workflow.
- Private connection to the deployment target.
- Deployment approval process.
- Automated status reporting.

### Exit criteria

- GitHub can deploy without exposing Raspberry Pi SSH publicly.
- Only approved tagged releases can reach production.
- Concurrent deployments are prevented.
- Failed deployments are reported and rolled back.
- Manual recovery remains possible.

---

## Sprint 15 — Observability and Operations

### Goal

Make the service easy to monitor, diagnose, and maintain.

### Main tasks

- Standardize structured logs.
- Configure journald and log rotation.
- Add service uptime metrics.
- Add queue depth and dropped-event metrics.
- Add cache and blocklist metrics.
- Add CPU, RAM, temperature, storage, and database-size monitoring.
- Add health status to the dashboard.
- Add backup verification.
- Add database integrity checks.
- Add maintenance commands.
- Create incident and recovery runbooks.

### Deliverables

- Operations dashboard section.
- Maintenance scripts.
- Log and metric documentation.
- Recovery runbooks.

### Exit criteria

- Service failures can be diagnosed from SSH and logs.
- Storage growth is visible.
- Dropped query events are detectable.
- Backups can be restored successfully.
- Database integrity is checked periodically.

---

## Sprint 16 — Performance and Reliability Validation

### Goal

Verify that the system meets its Raspberry Pi resource and reliability targets.

### Main tasks

- Build DNS load-test tools or scripts.
- Measure upstream and cached latency.
- Measure filtering lookup time.
- Measure blocklist loading time and memory.
- Measure query logging throughput.
- Test dashboard queries at target data volume.
- Test long-running stability.
- Test service restart behavior.
- Test database locking and recovery.
- Test low-disk and upstream-failure scenarios.
- Tune cache, queue, batch, and SQLite settings.
- Document final resource requirements.

### Deliverables

- Benchmark report.
- Resource-usage report.
- Reliability test report.
- Tuned production defaults.

### Exit criteria

- Normal active memory remains within the defined target.
- DNS remains responsive during dashboard activity.
- Queue and database failures do not stop DNS.
- Cache and filtering targets are met.
- A multi-day stability test completes without critical failure.

---

## Sprint 17 — Documentation and Version 1.0

### Goal

Prepare the project for public presentation and repeatable installation.

### Main tasks

- Complete README and project screenshots.
- Document architecture.
- Document DNS behavior and rule priority.
- Create installation guide.
- Create configuration reference.
- Create API guide.
- Create development guide.
- Create CI/CD guide.
- Create backup and recovery guide.
- Add troubleshooting section.
- Add contribution guidelines.
- Add project demo data.
- Review licensing and third-party notices.
- Tag and publish version 1.0.

### Deliverables

- Complete project documentation.
- Public version 1.0 release.
- Portfolio-ready presentation.
- Installation and recovery guides.

### Exit criteria

- A new user can install the project using the documentation.
- A contributor can run tests and submit a pull request.
- Production configuration is fully described.
- Recovery from a failed release or database issue is documented.
- Version 1.0 is tagged and published.

---

# 11. Release Milestones

## Milestone 0 — Repository Ready

Includes:

- Sprint 0

Result:

- Structured repository and engineering conventions.

## Milestone 1 — DNS MVP

Includes:

- Sprints 1–2

Result:

- Raspberry Pi resolves DNS requests through the custom Go service.

## Milestone 2 — Filtering Resolver

Includes:

- Sprints 3–4

Result:

- DNS filtering and caching work in memory.

## Milestone 3 — Persistent Appliance

Includes:

- Sprint 5

Result:

- Queries, rules, clients, and aggregates persist in SQLite.

## Milestone 4 — Management Platform

Includes:

- Sprints 6–9

Result:

- Secure FastAPI API and responsive React dashboard.

## Milestone 5 — DNS Intelligence

Includes:

- Sprint 10

Result:

- Historical analytics, exports, and network insights.

## Milestone 6 — Production Delivery

Includes:

- Sprints 11–14

Result:

- CI, releases, SSH deployment, rollback, and protected CD.

## Milestone 7 — Version 1.0

Includes:

- Sprints 15–17

Result:

- Monitored, benchmarked, documented, portfolio-ready release.

---

# 12. Definition of Done

A task is done when:

- implementation is complete;
- automated tests cover expected behavior and relevant errors;
- linting and formatting pass;
- documentation is updated;
- configuration changes include examples;
- migrations are included when required;
- logging is sufficient for diagnosis;
- security impact has been considered;
- performance impact has been considered;
- the pull request is reviewed;
- CI passes.

A sprint is done when:

- all mandatory deliverables exist;
- all exit criteria pass;
- known limitations are documented;
- unfinished work is moved explicitly to the backlog;
- the sprint document is updated with the final outcome.

---

# 13. Testing Strategy

## Go DNS service

- Unit tests for domain normalization and matching.
- Unit tests for cache expiration and eviction.
- Unit tests for configuration.
- Integration tests with a local upstream DNS server.
- UDP and TCP resolution tests.
- Concurrency tests.
- Race detection.
- Benchmarks for rule lookup and cache access.

## FastAPI

- Repository tests.
- Service tests.
- API endpoint tests.
- Authentication tests.
- Migration tests.
- Analytics correctness tests.
- DNS control client tests.

## React

- Component tests.
- Page-level interaction tests.
- Loading and failure-state tests.
- Form validation tests.
- API mocking.
- Production build validation.

## System

- DNS resolution through the Raspberry Pi.
- Blocking and whitelist override tests.
- Database write and dashboard read concurrency.
- Restart and recovery tests.
- Deployment and rollback tests.
- Resource and load tests.

---

# 14. CI/CD Strategy

## Pull requests

Every pull request should run:

```text
Go
- formatting
- vet
- lint
- tests
- race tests where practical
- build

Python
- Ruff lint
- Ruff format check
- type check
- pytest
- migration validation

React
- ESLint
- Vitest
- production build

Repository
- secret scan
- dependency scan
- configuration validation
```

## Main branch

A successful merge should:

- repeat validation;
- build ARM64 artifacts;
- build the React production bundle;
- package all components;
- retain build artifacts.

## Tagged release

A version tag should:

- build reproducible packages;
- generate checksums;
- publish a GitHub release;
- generate release notes.

## Production deployment

The initial deployment method will be manual from the development laptop:

```text
GitHub Actions creates release
        |
Developer downloads or selects release
        |
Deployment script uploads through local SSH
        |
Pi backs up database
        |
Pi installs versioned release
        |
Migrations run
        |
Services restart
        |
Health checks run
        |
Automatic rollback on failure
```

Automated CD will be introduced only after manual deployment is reliable.

---

# 15. Initial API Scope

Planned routes include:

```text
Authentication
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me

Health
GET    /api/health

Dashboard
GET    /api/stats/overview
GET    /api/stats/timeseries
GET    /api/stats/top-domains
GET    /api/stats/top-blocked
GET    /api/stats/top-clients
GET    /api/stats/cache
GET    /api/stats/upstreams

Queries
GET    /api/queries
GET    /api/queries/export

Clients
GET    /api/clients
GET    /api/clients/{id}
PATCH  /api/clients/{id}

Rules
GET    /api/rules
POST   /api/rules
PATCH  /api/rules/{id}
DELETE /api/rules/{id}

Blocklist sources
GET    /api/blocklists
POST   /api/blocklists
PATCH  /api/blocklists/{id}
DELETE /api/blocklists/{id}
POST   /api/blocklists/{id}/update
POST   /api/blocklists/update-all

DNS controls
GET    /api/dns/status
POST   /api/dns/reload-rules
POST   /api/dns/flush-cache

Settings
GET    /api/settings
PATCH  /api/settings
```

The detailed API contract will be defined during Sprint 6.

---

# 16. Dashboard Scope

Initial pages:

- Login
- Overview
- Query explorer
- Clients
- Client details
- Blacklist
- Whitelist
- Blocklist sources
- Settings
- System health

Initial overview metrics:

- queries today;
- blocked queries and percentage;
- active clients;
- average response latency;
- cache hit ratio;
- queries over time;
- top requested domains;
- top blocked domains;
- top clients;
- upstream status.

Later intelligence metrics:

- newly observed domains;
- query-rate anomalies;
- NXDOMAIN trends;
- blocklist contribution;
- per-device behavioral changes;
- upstream latency comparison.

---

# 17. Risks and Mitigations

## Raspberry Pi memory limits

**Risk:** Large Python sets, caches, or frontend tooling could exhaust RAM.

**Mitigation:**

- Go for the DNS path;
- static React deployment;
- one Uvicorn worker;
- bounded cache and event queues;
- measured blocklist limits;
- no Docker initially.

## microSD wear

**Risk:** Frequent database and log writes reduce storage lifetime.

**Mitigation:**

- batched SQLite writes;
- WAL mode;
- retention and aggregation;
- controlled logging;
- database backups;
- optional external USB storage.

## SQLite write contention

**Risk:** DNS logging and API updates compete for the writer lock.

**Mitigation:**

- one batched DNS writer;
- short transactions;
- WAL mode;
- retry and busy timeout;
- avoid synchronous DNS-path database access.

## DNS outage

**Risk:** A bug can disrupt the entire home network.

**Mitigation:**

- staged testing before router-wide adoption;
- reliable systemd restart;
- health checks;
- fallback resolver documentation;
- secondary DNS strategy where appropriate;
- rollback-ready deployments.

## Remote deployment exposure

**Risk:** Publicly exposing SSH creates unnecessary attack surface.

**Mitigation:**

- local manual deployment first;
- private VPN or self-hosted runner later;
- SSH keys only;
- no router port forwarding.

## Scope expansion

**Risk:** Advanced analytics delay the core resolver.

**Mitigation:**

- strict milestone order;
- DNS MVP before dashboard;
- clear sprint exit criteria;
- optional features placed after core functionality.

---

# 18. Out of Scope for Version 1.0

Unless added deliberately later, version 1.0 will not include:

- a recursive DNS resolver implemented from scratch;
- full DNSSEC validation;
- DNS-over-HTTPS server support;
- DNS-over-TLS server support;
- multi-node clustering;
- high availability;
- cloud-hosted account management;
- mobile applications;
- enterprise role-based access control;
- machine-learning threat classification;
- deep packet inspection;
- support for networks beyond the home or small lab scope.

Encrypted upstream DNS may be considered after version 1.0.

---

# 19. Future Roadmap

Potential version 2 features:

- per-client policy profiles;
- scheduled blocking profiles;
- category-based filtering;
- encrypted upstream DNS;
- multiple upstream selection strategies;
- optional Prometheus metrics;
- notification integrations;
- domain reputation enrichment;
- anomaly scoring;
- backup DNS node support;
- configuration import and export;
- plugin system for analytics modules;
- optional compact native blocklist storage;
- multilingual dashboard.

---

# 20. Documentation Plan

Each sprint document should contain:

1. Sprint purpose
2. Context and dependencies
3. Architecture decisions
4. User stories
5. Functional requirements
6. Non-functional requirements
7. Detailed tasks
8. Suggested implementation sequence
9. Data structures or API contracts
10. Test plan
11. Security considerations
12. Performance considerations
13. Deliverables
14. Acceptance criteria
15. Validation commands
16. Known risks
17. Completion notes

This roadmap defines **what** will be delivered and in what order.

The individual sprint documents will define **how** each sprint should be implemented.
