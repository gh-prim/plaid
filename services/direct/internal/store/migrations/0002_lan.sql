-- The server's private IPv4 and HTTPS port on its own network, so a short
-- link opened from that network can skip the router (no NAT loopback needed).
ALTER TABLE servers ADD COLUMN lan_ip TEXT NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN lan_port INTEGER NOT NULL DEFAULT 0;
