# UDP vs TCP for DNS

UDP is the default transport because it is lightweight.

TCP is required for:
- Large responses
- Truncated UDP replies
- Some DNSSEC scenarios

HomeDNS will support both from version 0.1.
