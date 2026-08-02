# Installation

## Hardware

-   Raspberry Pi 3B
-   16 GB+ microSD
-   Ethernet connection
-   Stable power supply

## Operating System

-   Debian / Raspberry Pi OS Lite
-   SSH enabled
-   mDNS enabled (`homedns.local`)

## Required packages

``` bash
sudo apt update
sudo apt install git curl jq tar make
```

## Users

-   `joyteaser` (administration)
-   `homedns` (service account)

## Validation

``` bash
hostnamectl
systemctl status homedns-dns
curl http://127.0.0.1:8081/health | jq
dig @192.168.1.32 example.com
vcgencmd get_throttled
```
