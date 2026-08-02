# Running HomeDNS

## Service status

``` bash
systemctl status homedns-dns
```

## Logs

``` bash
journalctl -fu homedns-dns
```

## Version

``` bash
/opt/homedns/current/homedns-dns --version
```

## DNS test

``` bash
dig @192.168.1.32 example.com
```
