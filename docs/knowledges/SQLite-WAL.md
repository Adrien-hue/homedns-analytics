# SQLite WAL

HomeDNS uses SQLite in WAL mode because it allows concurrent readers while a writer appends transactions.

DNS logging will batch writes to reduce SD-card wear and lock contention.
