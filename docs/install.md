# Installing PLAID

Three ways to run the server. All of them end the same way: open `http://<your-server>:32500`.

| | Best for |
|---|---|
| [Native package (Ubuntu/Debian)](#native-package-ubuntu--debian) | A home server where PLAID should start on boot, like Plex. No Docker needed. |
| [Install script](#install-script) | Same as above, in one command. |
| [Docker](#docker) | Machines already running Docker / Compose. |

Supported: Ubuntu 24.04 LTS and 26.04 LTS, Debian 12+, on `amd64` (most PCs) and `arm64` (Raspberry Pi 4/5 with a 64-bit OS, ARM servers).

> PLAID is in early development: there is no published release yet. Until there is, build the packages yourself with `make deb` (they land in `dist/`).

## Native package (Ubuntu / Debian)

1. Download `plaid_<version>_amd64.deb` (or `_arm64.deb`) from the [releases page](https://github.com/gh-prim/plaid/releases), and `checksums.txt`.
2. Optionally check it: `sha256sum -c --ignore-missing checksums.txt`
3. Install:
   ```bash
   sudo apt install ./plaid_<version>_amd64.deb
   ```

What you get:

| Path | What |
|------|------|
| `/usr/bin/plaid` | The server |
| `/etc/plaid/plaid.env` | Configuration (port, data dir, logs). **Kept on upgrades.** |
| `/var/lib/plaid` | Data: database, caches. **Never deleted automatically.** |
| `plaid.service` | systemd service, enabled and started, runs as the `plaid` user |

Everyday commands:

```bash
systemctl status plaid          # is it running?
journalctl -u plaid -f          # live logs
sudo systemctl restart plaid    # after editing /etc/plaid/plaid.env
```

**Upgrade:** install the newer `.deb` the same way (or re-run the install script). The service restarts on the new version; your config and data stay.

**Uninstall:**
- `sudo apt remove plaid`: stops and removes PLAID, keeps config and data.
- `sudo apt purge plaid`: also removes `/etc/plaid`. Your data in `/var/lib/plaid` is still kept; delete it yourself if you really want to.

## Install script

```bash
curl -fsSL https://raw.githubusercontent.com/gh-prim/plaid/main/packaging/install.sh | sudo bash
```

It detects your CPU, downloads the latest `.deb` and `checksums.txt` from GitHub, refuses to continue if the checksum doesn't match, then runs `apt install`. That's all: [read it](../packaging/install.sh) first if you like, it's short.

## Docker

```bash
git clone https://github.com/gh-prim/plaid.git && cd plaid
docker compose up -d
```

Edit `docker-compose.yml` to mount your media folders read-only (`/path/to/movies:/media/movies:ro`). Data lives in the `plaid-data` volume. The container runs as a non-root user (uid 1000).

## Configuration

All settings are environment variables (in `/etc/plaid/plaid.env` for the native package, or `environment:` in Compose):

| Variable | Default | |
|----------|---------|---|
| `PLAID_PORT` | `32500` | HTTP port |
| `PLAID_HOST` | `0.0.0.0` | Listen address (`127.0.0.1` = this machine only) |
| `PLAID_DATA_DIR` | `/var/lib/plaid` (package), `/data` (Docker) | Database and caches |
| `PLAID_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PLAID_LOG_FORMAT` | `text` (package), `json` (Docker) | `text` or `json` |

## Letting PLAID read your media folders

This is the most common setup issue on Linux (Plex users know it well). The native package runs PLAID as the `plaid` system user, which **can't read your personal folders by default**, for example `/home/you/Movies`.

Give it read access with one of these, then `sudo systemctl restart plaid`:

**Option A: share a group (simplest).** If your media belongs to a group (check with `ls -ld /path/to/media`):

```bash
sudo usermod -aG <that-group> plaid
```

The folders need to be group-readable (`chmod -R g+rX /path/to/media`), and every parent folder needs to be group-traversable.

**Option B: ACLs (no change to existing permissions).**

```bash
sudo apt install acl
sudo setfacl -R -m u:plaid:rX /path/to/media
sudo setfacl -R -d -m u:plaid:rX /path/to/media    # also for files added later
# Each parent folder must be traversable, e.g. for /home/you/Movies:
sudo setfacl -m u:plaid:x /home/you
```

Check it worked: `sudo -u plaid ls /path/to/media`.

PLAID only ever needs **read** access to media; it never writes to your library folders.
