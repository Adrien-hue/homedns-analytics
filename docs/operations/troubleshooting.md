# Troubleshooting

## Health endpoint unavailable

Check:

``` bash
systemctl status homedns-dns
journalctl -u homedns-dns
```

## SSH unavailable

-   Check Ethernet
-   Verify IP
-   Ping `homedns.local`

## Deployment rollback

Inspect service logs and current release symlink.

## Benchmark degraded

Review timeout rate, failure rate and TCP concurrent scenario.
