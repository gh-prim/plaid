# getplaid.direct protocol (v1)

`getplaid.direct` gives every PLAID server that opts in a stable hostname, so
people outside the home network reach it without typing an IP and a port.

```
<slug>-<id>.getplaid.direct            A/AAAA  -> the server's public IP
<a-b-c-d>.<slug>-<id>.getplaid.direct  A       -> a private LAN IP (a.b.c.d), for clients at home
_acme-challenge.<slug>-<id>...         TXT     -> set by the server while it gets its certificate
```

- `slug`: the server name lowercased, every run of characters outside
  `[a-z0-9]` turned into one `-`, trimmed of `-`, cut to 20 characters;
  `plaid` when nothing is left.
- `id`: 6 random characters from `[a-z2-7]` (base32, lowercase), unique.
  The `<slug>-<id>` label is the server's **label**; the id alone identifies it.

The service never sees media traffic: DNS points straight at the home server,
and each server gets its own Let's Encrypt certificate through DNS-01 (its
private key never leaves the home server). The service stores, per server:
id, slug, token hash (SHA-256), public IPv4/IPv6, external port, PLAID
version, created/last-seen times, and current ACME TXT values. Nothing else.

## HTTP API

Base URL `https://getplaid.direct/api/v1`. JSON in and out. Errors:
`{"error": "<code>", "message": "<english text>"}` with a 4xx/5xx status.

The server's public IP is **always the source address of the request**
(behind the reverse proxy: the left-most `X-Forwarded-For` entry added by a
trusted proxy). A client can't point a name at an address it doesn't own.

### `POST /servers` — register
Body: `{"name": "Living room", "port": 32501, "version": "0.3.0"}`
`port` is the external port the router forwards to the server's HTTPS listener.

`201`:
```json
{
  "id": "k3x7qa",
  "token": "<43-char random secret, shown only once>",
  "hostname": "living-room-k3x7qa.getplaid.direct",
  "url": "https://living-room-k3x7qa.getplaid.direct:32501",
  "shortUrl": "https://getplaid.direct/living-room-k3x7qa",
  "publicIp": "203.0.113.7",
  "port": 32501
}
```
Rate limit: 5 registrations per source IP per hour (`429 rate_limited`).

All other calls carry `Authorization: Bearer <token>`; a wrong token or
unknown id is `401 unauthorized` (never reveal which one).

### `PUT /servers/{id}` — update and heartbeat
Body: `{"name": "Living room", "port": 32501, "version": "0.3.0"}` (all
optional; the name changes the slug, so the hostname too). Refreshes the
public IP from the request and `lastSeen`. The server calls it at startup,
every 15 minutes and whenever its port changes. `200` with the same shape as
registration, minus `token`.

### `DELETE /servers/{id}` — unregister
`204`. Records disappear at once.

### `PUT /servers/{id}/acme-challenge` — set DNS-01 values
Body: `{"values": ["<base64url digest>", "..."]}` (1 to 4 values: one per
authorization, a certificate for `host` and `*.host` needs two). Replaces
the TXT values of `_acme-challenge.<hostname>`. `204`.

### `DELETE /servers/{id}/acme-challenge`
Clears them. `204`. Values also expire on their own after 1 hour.

### `POST /servers/{id}/reachability` — check from outside
The service opens `https://<publicIp>:<port>/api/v1/health` (TLS not
verified, 5 s timeout) and reports whether a PLAID server answered
(a JSON body with a `version` field):
`200 {"reachable": true, "publicIp": "203.0.113.7", "port": 32501}`
or `{"reachable": false, ..., "error": "timeout|refused|not_plaid|tls"}`.
Rate limit: 10 per server per hour.

### `GET /ip`
No auth. `200 {"ip": "203.0.113.7"}`: what the service sees as the caller's
address.

## Short links

`GET https://getplaid.direct/<label>` answers `302` to
`https://<label>.getplaid.direct:<port>/`, so a person only ever has to
remember or click one link. Unknown label: `404` page that links to
getplaid.tv.

## DNS

The service is the authoritative name server for `getplaid.direct`
(registrar NS records: `ns1.getplaid.direct`, `ns2.getplaid.direct`, with
glue to the service's IP). It answers:

| Name | Type | Answer |
|------|------|--------|
| `getplaid.direct` | SOA, NS | the service itself |
| `getplaid.direct`, `ns1`, `ns2` | A | the service's public IP (config) |
| `getplaid.direct` | CAA | `0 issue "letsencrypt.org"` |
| `<label>` | A / AAAA | the server's last public IPv4 / IPv6 |
| `<a-b-c-d>.<label>` | A | `a.b.c.d`, only for RFC 1918 / CGNAT / link-local addresses, only under a registered label |
| `_acme-challenge.<label>` | TXT | current DNS-01 values |

TTL 60 s for server records, 3600 s for the zone's own. Unknown names are
`NXDOMAIN`. Servers not seen for 30 days are removed.

## Details pinned down by the implementation

These complete the sections above; the PLAID server can rely on them.

- **Source address behind the proxy.** `X-Forwarded-For` is read only when
  the peer is a trusted proxy, and from the right: trusted hops are skipped
  and the first other entry is the client. Entries further left were written
  by the client and are ignored.
- **IPv4 and IPv6.** A request refreshes only the address of its own family
  (an IPv4 call updates the A record, an IPv6 call the AAAA record). An
  address not refreshed within 24 hours of `lastSeen` is no longer published,
  so a server that only heartbeats over IPv4 loses a stale AAAA.
- **Port 443.** `url` and the short-link redirect leave the port out when it
  is 443 (`https://living-room-k3x7qa.getplaid.direct`).
- **Validation.** Register: `port` is required (1 to 65535); `name` is
  optional (slug `plaid`), at most 200 bytes of UTF-8; `version` at most 40
  printable ASCII characters. Bodies are at most 8 KiB. ACME values are 1 to
  128 base64url characters (`[A-Za-z0-9_-]`).
- **Error codes.** Besides `unauthorized` and `rate_limited`:
  `400 bad_request`, `404 not_found` (unknown `/api/` path),
  `413 too_large`, `500 internal`.
- **Rate limits.** IPv6 registrations are counted per `/64`. Limits are kept
  in memory and reset when the service restarts.
- **Reachability.** The probe targets the caller's address (the request's
  source, as for `publicIp`) and the stored `port`. It never probes a
  loopback, private, CGNAT or link-local address: such a target answers
  `reachable: false, error: "refused"` without any connection. A plain-HTTP
  answer on the port is `tls`.
- **Short links.** The id alone identifies the server: a link with an old
  slug still redirects, to the current hostname. `GET /` redirects to
  getplaid.tv. DNS, on the other hand, only answers for the current label.
- **LAN names.** `a-b-c-d` uses plain decimal octets without leading zeros
  and answers only for 10/8, 172.16/12, 192.168/16, 100.64/10 (CGNAT) and
  169.254/16 (link-local). Anything else is `NXDOMAIN`.
- **DNS.** Negative answers (NXDOMAIN, and NODATA for an existing name with
  no record of that type) carry the SOA, whose minimum TTL is 60 s. Queries
  outside the zone are `REFUSED`: the service is not a resolver.
- **Health.** `GET /healthz` answers `200 ok` when the database responds.
