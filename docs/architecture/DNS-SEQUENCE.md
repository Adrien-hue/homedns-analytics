# DNS Request Sequence

```text
Client
  |
  | DNS Query
  v
Go DNS Server
  |
  +--> Parse packet
  |
  +--> Validate query
  |
  +--> (Future) Whitelist
  |
  +--> (Future) Blacklist
  |
  +--> (Future) Cache
  |
  +--> Forward upstream
            |
            v
     Upstream Resolver
            |
            v
Go DNS Server
  |
  +--> Return response
  |
  +--> Publish async event (future)
```

The forwarding path must remain synchronous. Logging and analytics are asynchronous.
