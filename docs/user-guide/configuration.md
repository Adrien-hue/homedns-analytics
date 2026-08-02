# Configuration

The DNS service is configured through the YAML configuration file.

Typical settings include:

-   DNS listen address
-   Upstream DNS server
-   Query timeout
-   Health endpoint

After changing the configuration:

``` bash
sudo systemctl restart homedns-dns
```

Always verify:

``` bash
curl http://127.0.0.1:8081/health | jq
```
