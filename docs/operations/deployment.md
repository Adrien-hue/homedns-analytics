# Deployment

## Build

``` bash
make release-package
```

## Deploy

``` bash
make deploy
```

## Validate

``` bash
make deploy-info
systemctl status homedns-dns
curl http://127.0.0.1:8081/health | jq
dig @192.168.1.32 example.com
```

## Release layout

    /opt/homedns/
    ├── current
    ├── releases/
    └── shared/

Deployments automatically validate the release and rollback on failure.
