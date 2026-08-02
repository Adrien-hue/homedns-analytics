# Upgrading HomeDNS

1.  Pull the latest code.
2.  Build a release package.
3.  Deploy the release.

``` bash
git pull
make release-package
make deploy
```

The deployment process validates the new release and automatically rolls
back if the health check fails.
