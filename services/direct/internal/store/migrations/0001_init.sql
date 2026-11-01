-- Everything the service knows about a server. Times are Unix seconds.
CREATE TABLE servers (
    id         TEXT PRIMARY KEY,
    slug       TEXT NOT NULL,
    token_hash BLOB NOT NULL,       -- SHA-256 of the bearer token
    ipv4       TEXT NOT NULL DEFAULT '',
    ipv4_seen  INTEGER NOT NULL DEFAULT 0,
    ipv6       TEXT NOT NULL DEFAULT '',
    ipv6_seen  INTEGER NOT NULL DEFAULT 0,
    port       INTEGER NOT NULL,
    version    TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    last_seen  INTEGER NOT NULL
) STRICT;

CREATE INDEX servers_last_seen ON servers (last_seen);

-- Current DNS-01 values of _acme-challenge.<label>.
CREATE TABLE acme_values (
    server_id  TEXT NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    value      TEXT NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE INDEX acme_values_server ON acme_values (server_id);
CREATE INDEX acme_values_expires ON acme_values (expires_at);
