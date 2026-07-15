# SPRINT-00 — Project Foundation

> **Sprint duration:** 1–2 days

## Goal

Build the engineering foundation of HomeDNS Analytics before writing production code.

## Objectives

- Create the repository structure.
- Define coding conventions.
- Define Git workflow.
- Prepare documentation.
- Add the first CI workflow.
- Configure development tooling.

## Exit Criteria

- [ ] GitHub repository created
- [ ] README.md completed
- [ ] ROADMAP.md committed
- [ ] Sprint documentation folder created
- [ ] Folder structure committed
- [ ] LICENSE selected
- [ ] .gitignore created
- [ ] .editorconfig created
- [ ] .pre-commit-config.yaml created
- [ ] Makefile created
- [ ] Placeholder GitHub Actions workflow passes

## Repository Structure

```text
homedns/
├── .github/
├── backend/
├── dns/
├── frontend/
├── deploy/
├── docs/
│   ├── architecture/
│   ├── decisions/
│   ├── operations/
│   └── sprints/
├── scripts/
├── README.md
├── ROADMAP.md
├── Makefile
└── CONTRIBUTING.md
```

## Branch Strategy

- main
- develop
- feature/*
- fix/*
- docs/*

Rules:

- No direct pushes to `main`
- Pull Requests required
- Squash merge enabled

## Commit Convention

Use Conventional Commits:

- feat
- fix
- docs
- refactor
- test
- ci
- chore

## Tooling

### Go

- gofmt
- go vet
- golangci-lint

### Python

- Ruff
- pytest

### Frontend

- ESLint
- Vitest

## Make Targets

- make help
- make lint
- make test
- make build
- make ci
- make clean

## Documentation

Create:

- docs/sprints/
- docs/architecture/
- docs/decisions/
- docs/operations/

Create ADRs:

- ADR-001 Project Stack
- ADR-002 SQLite
- ADR-003 Go DNS

## GitHub

Configure:

- Branch protection
- Required CI
- PR template
- Issue templates
- Labels
- Milestones

## Deliverables

- Professional repository
- CI skeleton
- Documentation structure
- Coding conventions
- Backlog initialized

## Next Sprint

Sprint 01 prepares the Raspberry Pi infrastructure, SSH access, and runtime environment.
