# getplaid.direct

The service behind `getplaid.direct`: it gives every PLAID server that opts in
a stable hostname (`living-room-k3x7qa.getplaid.direct`), a short link
(`https://getplaid.direct/living-room-k3x7qa`) and the DNS-01 records it needs
to get its own Let's Encrypt certificate. Media never goes through it: DNS
points straight at the home server.

One Go binary does three things:

- the HTTP API used by PLAID servers (`/api/v1`), plus short links and `/healthz`;
- the authoritative DNS server for the zone (UDP and TCP);
- a janitor that removes servers unseen for 30 days and expired ACME values.

The contract with the PLAID server is [PROTOCOL.md](PROTOCOL.md). State is one
SQLite file (`direct.db`) in the data directory.

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `DIRECT_ZONE` | `getplaid.direct` | The zone served, and the API's host |
| `DIRECT_PUBLIC_IP` | none, **required** | The service's own public IPv4: apex, `ns1`, `ns2` A records and glue |
| `DIRECT_PUBLIC_IPV6` | none | The service's own public IPv6, if it has one |
| `DIRECT_HTTP_ADDR` | `:8080` | Plain HTTP listener (TLS is terminated by the reverse proxy) |
| `DIRECT_DNS_ADDR` | `:53` (`:5353` in the image) | DNS listener, UDP and TCP; `off` disables DNS (then `DIRECT_PUBLIC_IP` is optional) |
| `DIRECT_DATA_DIR` | `./data` (`/data` in the image) | Where `direct.db` lives |
| `DIRECT_TRUSTED_PROXIES` | loopback, RFC 1918, `fc00::/7` | Comma-separated CIDRs (or addresses) whose `X-Forwarded-For` is believed |
| `DIRECT_SITE_URL` | `https://getplaid.tv` | Where `/` redirects and the short-link 404 page links |
| `DIRECT_ADMIN_EMAIL` | `hostmaster@<zone>` | SOA contact (RNAME) |

Logs are JSON lines on stdout. Tokens are never logged; only their SHA-256 is stored.

## Run locally

```bash
cd services/direct
DIRECT_PUBLIC_IP=127.0.0.1 DIRECT_DNS_ADDR=127.0.0.1:5353 go run ./cmd/direct
```

Register a server and look at what it gets:

```bash
curl -s -X POST localhost:8080/api/v1/servers \
  -d '{"name":"Living room","port":32501,"version":"0.3.0"}'
# {"id":"k3x7qa","token":"…","hostname":"living-room-k3x7qa.getplaid.direct",…}

curl -si localhost:8080/living-room-k3x7qa | grep -i location
```

Query the DNS server:

```bash
dig @127.0.0.1 -p 5353 getplaid.direct SOA
dig @127.0.0.1 -p 5353 getplaid.direct NS
dig @127.0.0.1 -p 5353 living-room-k3x7qa.getplaid.direct A
dig @127.0.0.1 -p 5353 192-168-1-20.living-room-k3x7qa.getplaid.direct A
dig @127.0.0.1 -p 5353 _acme-challenge.living-room-k3x7qa.getplaid.direct TXT
dig @127.0.0.1 -p 5353 +tcp getplaid.direct CAA
```

Requests from your own machine come from `127.0.0.1`, so that is the address a
locally registered server gets.

## Test

```bash
cd services/direct
go vet ./...
go test -race ./...
golangci-lint run ./...   # same rules as server/
```

The DNS tests start the server on a random UDP/TCP port and query it with a
real client; the API tests run end to end through `httptest`; the probe tests
use local TLS servers.

## Docker

```bash
docker build -t plaid-direct .
docker run --rm -e DIRECT_PUBLIC_IP=203.0.113.10 \
  -p 8080:8080 -p 53:5353/udp -p 53:5353/tcp -v direct-data:/data plaid-direct
```

The image is distroless and runs as uid 65532. It listens on 5353 inside the
container so it needs no capability to bind; publish the host's port 53 (UDP
and TCP) to it. `/direct -healthcheck` is the container health check.

## Deploy on Coolify

The service runs on a host managed by Coolify, which already runs Traefik.

### 1. The host

- **Free port 53**, or set `DIRECT_DNS_BIND` to the host's public IP: systemd-resolved only holds `127.0.0.53:53`, so binding the public address alone avoids the clash without touching the resolver. Otherwise: On Ubuntu/Debian, systemd-resolved's stub listener holds
  `127.0.0.53:53` and blocks Docker from publishing `0.0.0.0:53`. In
  `/etc/systemd/resolved.conf` set `DNSStubListener=no`, then
  `sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf` and
  `sudo systemctl restart systemd-resolved`. Check with `sudo ss -lunpt | grep ':53 '`.
- **Firewall.** Open 53/udp and 53/tcp (TCP is needed for large answers and
  by some resolvers), plus 80 and 443 for Traefik. With ufw:
  `sudo ufw allow 53/udp && sudo ufw allow 53/tcp`. Do the same in the cloud
  provider's firewall if there is one.

### 2. The registrar

The service is the zone's authoritative server, so the registrar only
delegates to it:

1. Create two **host (glue) records**: `ns1.getplaid.direct` and
   `ns2.getplaid.direct`, both pointing to the host's public IPv4 (and IPv6
   if it has one). Registrars call these "child name servers", "glue records"
   or "register name server".
2. Set the domain's **name servers** to `ns1.getplaid.direct` and
   `ns2.getplaid.direct`.

Both names point to the same machine: registrars require two name servers,
but there is only one. Adding a secondary (another host running the same
image against a replicated database, or a secondary DNS provider) is future
work. Delegation can take up to the parent zone's TTL (often 1 to 48 h).
Check it with `dig +trace getplaid.direct NS`.

### 3. The Coolify resource

1. New resource → your repository → build pack **Docker Compose**, base
   directory `/services/direct`, compose file `docker-compose.yml`.
2. Environment: set `DIRECT_PUBLIC_IP` (the host's public IPv4, the glue
   record's address), and optionally `DIRECT_PUBLIC_IPV6`, `DIRECT_ADMIN_EMAIL`.
3. Domains: the compose file declares `SERVICE_FQDN_DIRECT_8080`, Coolify's
   convention for "route this domain to container port 8080". In the
   `direct` service's **Domains** field, set `https://getplaid.direct`.
   Coolify then writes the Traefik labels and Traefik gets a Let's Encrypt
   certificate for it (HTTP-01 works because the zone's apex A record points
   to the host, and the zone's CAA allows Let's Encrypt).
4. Deploy, then check: `curl https://getplaid.direct/healthz` and
   `dig @<host-ip> getplaid.direct SOA`.

The API sees clients through Traefik: the Docker network ranges are in the
default `DIRECT_TRUSTED_PROXIES`, so the client address is taken from the
`X-Forwarded-For` header Traefik adds. Don't publish port 8080 on the host:
a request reaching it directly from a private range could set that header
itself.

Port 53 is published by the compose file itself (`53:5353/udp`,
`53:5353/tcp`), not by Traefik: DNS doesn't go through the proxy.

### Let's Encrypt rate limits

Every PLAID server gets its own certificate for `<label>.getplaid.direct`,
and Let's Encrypt allows **50 new certificates per registered domain per
week**. `getplaid.direct` counts as one registered domain, so all PLAID
servers together share those 50 (renewals of an identical set of names are
exempt). This is fine for a beta, not for a launch.

The way out is to get `getplaid.direct` on the
[Public Suffix List](https://publicsuffix.org/): each `<label>.getplaid.direct`
then counts as its own registered domain, as for `duckdns.org` or `nip.io`.
Apply early through a pull request to
[publicsuffix/list](https://github.com/publicsuffix/list) (private domains
section, with a `_psl` TXT record proving control): review takes weeks, and
browsers and Let's Encrypt pick up the change in their next updates.
Until then, Let's Encrypt can raise the limit on request (rate-limit
adjustment form).
