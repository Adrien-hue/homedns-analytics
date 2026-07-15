
# SPRINT-01 — Raspberry Pi Infrastructure

> **Sprint duration:** 1–2 days

## Goal

Prepare a secure, lightweight and production-ready Raspberry Pi environment that will host HomeDNS Analytics.

This sprint focuses **only on infrastructure**. No DNS logic is implemented yet.

---

# Objectives

- Install Raspberry Pi OS Lite (64-bit)
- Configure secure SSH access
- Update and harden the system
- Create the application user
- Prepare the filesystem layout
- Install required runtimes
- Configure system services
- Record baseline performance

---

# User Stories

### As a developer

I want to SSH into the Raspberry Pi securely so that I never need a monitor or keyboard.

### As the application

I need dedicated directories and permissions so future deployments are predictable.

### As an operator

I want the Pi to reboot automatically into a known-good state.

---

# Deliverables

- Raspberry Pi OS Lite installed
- SSH key authentication enabled
- Password login disabled (after verification)
- Static IP or DHCP reservation configured
- Dedicated `homedns` user
- `/opt/homedns` directory structure
- Required packages installed
- Basic firewall configured
- Baseline metrics documented

---

# Tasks

## 1. Flash Raspberry Pi OS Lite

- Download Raspberry Pi OS Lite (64-bit)
- Flash using Raspberry Pi Imager
- Enable SSH during installation
- Configure hostname: `homedns`

---

## 2. First Boot

Verify:

- Internet connectivity
- SSH access
- Hostname
- Time synchronization

Commands:

```bash
hostnamectl
ip addr
timedatectl
```

---

## 3. Update the System

```bash
sudo apt update
sudo apt full-upgrade -y
sudo reboot
```

---

## 4. SSH Hardening

- Generate ED25519 key pair
- Copy public key
- Test key authentication
- Disable password login
- Disable root login

---

## 5. Create Application User

```bash
sudo adduser homedns
```

Permissions:

- no sudo by default
- dedicated home
- service ownership

---

## 6. Directory Layout

```text
/opt/homedns/
├── releases/
├── current/
├── shared/
│   ├── database/
│   ├── config/
│   └── logs/
└── backups/
```

---

## 7. Install Dependencies

Install:

- git
- curl
- sqlite3
- python3
- python3-venv
- python3-pip
- golang
- build-essential
- make

---

## 8. Network

Configure:

- Static DHCP reservation (recommended)
- Verify DNS resolution
- Verify internet access

---

## 9. Systemd Preparation

Create placeholder services:

- homedns-dns.service
- homedns-api.service

They won't start yet but establish the deployment layout.

---

## 10. Baseline Measurements

Record:

```bash
free -h
df -h
htop
vcgencmd measure_temp
systemd-analyze
```

Document:

- RAM usage
- CPU usage
- Boot time
- Temperature
- Storage usage

These measurements will be compared again after Version 1.0.

---

# Security Checklist

- SSH keys only
- Root login disabled
- System updated
- Firewall enabled
- Dedicated application user
- Minimal installed packages

---

# Validation

The sprint is successful when:

- SSH works using keys
- Raspberry Pi reboots correctly
- Internet connectivity remains stable
- `homedns` user exists
- `/opt/homedns` structure exists
- Required packages are installed
- Baseline metrics are recorded

---

# GitHub Issues

Suggested issues:

- #1 Install Raspberry Pi OS Lite
- #2 Configure SSH
- #3 Harden SSH
- #4 Create application user
- #5 Prepare filesystem
- #6 Install runtimes
- #7 Record baseline metrics

---

# Suggested Branch

```text
feature/rpi-infrastructure
```

---

# Suggested Commits

```text
chore(rpi): initial raspberry setup
chore(security): harden ssh configuration
docs(sprint): complete sprint 01
```

---

# Definition of Done

- Infrastructure documented
- Validation completed
- Sprint notes updated
- Ready to start Sprint 02 (DNS Forwarder MVP)
