# Getting Started

## Requirements

-   Raspberry Pi 3B or newer
-   Debian / Raspberry Pi OS Lite
-   SSH access
-   Ethernet connection

## Clone the repository

``` bash
git clone https://github.com/Adrien-hue/homedns-analytics.git
cd homedns-analytics/dns
```

## Build

``` bash
make release-package
```

## Deploy

``` bash
make deploy
```

## Verify

``` bash
make deploy-info
curl http://127.0.0.1:8081/health | jq
dig @<raspberry-ip> example.com
```
