# Service Management

## Start

``` bash
sudo systemctl start homedns-dns
```

## Stop

``` bash
sudo systemctl stop homedns-dns
```

## Restart

``` bash
sudo systemctl restart homedns-dns
```

## Status

``` bash
systemctl status homedns-dns
```

## Logs

``` bash
journalctl -u homedns-dns
journalctl -fu homedns-dns
```

## Version

``` bash
/opt/homedns/current/homedns-dns --version
```

## Health

``` bash
curl http://127.0.0.1:8081/health | jq
```
