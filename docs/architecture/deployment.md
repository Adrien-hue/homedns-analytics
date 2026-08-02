# Deployment Architecture

> **Target:** Raspberry Pi 3B, Linux ARM64  
> **Supervisor:** `systemd`  
> **Install root:** `/opt/homedns`

---

## 1. Purpose

The deployment system installs versioned HomeDNS releases on the Raspberry Pi without deploying the Git repository or building application code on the device.

The design prioritizes:

- reproducible ARM64 builds;
- traceable release metadata;
- immutable versioned release directories;
- atomic activation;
- persistent shared data;
- health-based validation;
- automatic rollback.

Operational commands are documented separately in `docs/operations/`.

---

## 2. End-to-End Flow

```text
Developer workstation
        |
        | package-release.sh
        v
Linux ARM64 binary
        |
        v
Versioned tar.gz archive
        |
        | SCP
        v
Raspberry Pi temporary directory
        |
        | install-release.sh as root
        v
Validate -> Extract -> Activate -> Restart -> Health check
                                   |
                                   +--> Rollback on failure
```

The deployment is initiated through the repository Makefile, which delegates to shell scripts.

---

## 3. Release Packaging

The package script runs on the developer workstation.

Responsibilities:

1. verify the Git working tree is clean;
2. resolve release metadata;
3. cross-compile the DNS binary for Linux ARM64;
4. create a temporary staging directory;
5. copy the runtime payload;
6. write the `RELEASE` metadata file;
7. create the compressed archive under `dns/dist`;
8. clean the temporary staging directory.

### 3.1 Build target

```text
GOOS=linux
GOARCH=arm64
CGO_ENABLED=0
```

The result is a statically linked ARM64 executable suitable for the Raspberry Pi target.

### 3.2 Runtime payload

The Sprint 02 archive contains:

```text
./
├── homedns-dns
├── RELEASE
├── Makefile
└── scripts/
    └── benchmarks/
        ├── benchmark.sh
        └── collect-baseline.sh
```

Documentation and the Git repository are intentionally excluded.

### 3.3 Release metadata

The `RELEASE` file records:

```text
release_id
version
commit_sha
created_at
dns_binary
```

The release ID uses a timestamp format:

```text
YYYYMMDD-HHMMSS
```

This gives every deployment a unique directory and audit trail.

---

## 4. Transfer Layer

The deployment script transfers two files:

- the release archive;
- `install-release.sh`.

The transfer uses SSH/SCP and a configurable target host, user, and remote temporary directory.

The installer is copied separately so the archive remains a pure runtime payload.

If connectivity fails before transfer completes, the currently active release is unchanged.

---

## 5. Raspberry Pi Filesystem Layout

```text
/opt/homedns/
├── current -> /opt/homedns/releases/<release-id>
├── releases/
│   ├── <older-release-id>/
│   ├── <current-release-id>/
│   └── ...
└── shared/
    ├── config/
    ├── data/
    ├── logs/
    └── benchmarks/
```

### 5.1 Versioned releases

Each release is extracted into:

```text
/opt/homedns/releases/<release-id>
```

Release directories are owned by the dedicated `homedns` user and group.

They contain immutable application files for that release.

### 5.2 Current symlink

The active release is selected through:

```text
/opt/homedns/current
```

The service unit starts:

```text
/opt/homedns/current/homedns-dns
```

Changing the symlink changes the release used on the next service restart.

### 5.3 Shared state

Persistent files are not stored inside release directories.

The `shared` tree survives upgrades and rollbacks:

- `shared/config` — runtime configuration;
- `shared/data` — future persistent application data;
- `shared/logs` — application-owned log files where used;
- `shared/benchmarks` — benchmark history and baselines.

Every release receives a `benchmarks` symlink pointing to the shared benchmark directory.

---

## 6. Installation Validation

The installer validates both the archive and the extracted release.

### 6.1 Archive validation

Checks include:

- archive exists;
- archive is a valid gzip-compressed tar file;
- required `RELEASE` entry exists;
- required DNS binary exists;
- portable handling of archive paths with or without a leading `./`.

### 6.2 Metadata validation

Required metadata includes:

- valid timestamp-shaped release ID;
- version;
- commit SHA;
- DNS binary name.

### 6.3 Binary validation

The extracted binary must:

- exist;
- be executable;
- have the expected ARM64 Linux file type;
- return valid version output.

### 6.4 Permission normalization

The installer applies controlled ownership and modes:

- directories executable and traversable;
- regular files non-executable by default;
- shell scripts executable;
- DNS binary executable;
- release owned by `homedns:homedns`.

---

## 7. Atomic Activation

Activation uses a temporary symlink:

```text
current.next -> releases/<new-release-id>
```

The installer then atomically replaces `current` with `current.next`.

```text
Create next link
      |
      v
Validate link target
      |
      v
Atomic rename to current
```

This prevents `current` from temporarily pointing to an incomplete path.

---

## 8. Service Restart and Health Validation

After activation:

1. reset any failed systemd state;
2. restart `homedns-dns.service`;
3. poll the health endpoint;
4. require both HTTP success and JSON readiness;
5. declare installation successful only after readiness passes.

Health validation requires:

```json
{
  "status": "ok",
  "ready": true
}
```

The installer retries quietly during normal startup. Temporary connection refusals are expected and do not pollute deployment output.

Default health policy:

```text
15 attempts
1 second delay
2 second request timeout
```

---

## 9. Automatic Rollback

The installer records the previously active release before switching.

If restart or health validation fails:

```text
New release unhealthy
        |
        v
Reactivate previous release
        |
        v
Restart service
        |
        v
Validate previous release health
```

If a valid previous release exists and becomes healthy, rollback succeeds automatically.

If no valid previous release exists, the installer stops the service and reports the failure.

The failed release directory remains available for investigation unless explicitly removed by failure cleanup.

---

## 10. Benchmark Persistence

Benchmark data is stored outside release directories.

This prevents:

- benchmark history disappearing after upgrade;
- rollback selecting a release with stale or isolated benchmark files;
- release cleanup deleting performance evidence.

The installer can migrate benchmark history from older layouts into the shared benchmark directory.

---

## 11. Security Boundary

Deployment uses two identities:

### SSH deployment user

`joyteaser`:

- authenticates through SSH keys;
- copies archives and installers;
- invokes the installer through `sudo`;
- performs administrative inspection.

### Application user

`homedns`:

- owns released application files and shared runtime directories;
- runs `homedns-dns.service`;
- writes benchmark reports;
- belongs to the `video` group so `vcgencmd` can read Raspberry Pi throttling state.

The application does not run as root.

---

## 12. Failure Isolation

| Failure point | Effect on active release |
|---|---|
| Local build fails | No remote change |
| Archive creation fails | No remote change |
| Network or SCP fails | No remote change |
| Archive validation fails | No activation |
| Extraction fails | New directory removed, current unchanged |
| Binary validation fails | No activation |
| Service restart fails | Rollback attempted |
| Health check fails | Rollback attempted |
| Rollback fails | Service stopped and error reported |

This staged design ensures that most failures occur before the active symlink changes.

---

## 13. Current Limitations

- release retention is not yet automatically pruned;
- no cryptographic signature verification beyond transport security and metadata;
- deployment uses direct SSH rather than a remote agent;
- health validation checks service readiness but not a DNS query;
- the deploy script depends on the target Ethernet or network connection;
- SSH connectivity preflight and disk-space checks can be improved;
- group and device permissions required for resource monitoring should be fully documented in operations setup.

These are operational improvement points, not blockers for `v0.2.0`.
