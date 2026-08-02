# Frequently Asked Questions

## The health endpoint is unreachable.

Check:

``` bash
systemctl status homedns-dns
journalctl -u homedns-dns
```

## DNS queries fail.

Verify:

-   Network connectivity
-   Upstream DNS server
-   Configuration file

## A benchmark is degraded.

Inspect the report for timeout rates, failure rates and the
worst-performing scenario before comparing it with the baseline.
