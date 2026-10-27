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

The package depends on your distribution's `ffmpeg`, which apt installs with it: PLAID uses `ffprobe` to analyze media files.

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

It detects your CPU, downloads the latest `.deb` and `checksums.txt` from GitHub, refuses to continue if the checksum doesn't match, refreshes apt's package lists (for `ffmpeg`), then runs `apt install`. That's all: [read it](../packaging/install.sh) first if you like, it's short.

## Docker

```bash
git clone https://github.com/gh-prim/plaid.git && cd plaid
docker compose up -d
```

Edit `docker-compose.yml` to mount your media folders read-only (`/path/to/movies:/media/movies:ro`), then use the paths **inside the container** (`/media/movies`) when you create a library. Data lives in the `plaid-data` volume. The container runs as a non-root user (uid 1000) and ships FFmpeg.

## Configuration

All settings are environment variables (in `/etc/plaid/plaid.env` for the native package, or `environment:` in Compose):

| Variable | Default | |
|----------|---------|---|
| `PLAID_PORT` | `32500` | HTTP port |
| `PLAID_HOST` | `0.0.0.0` | Listen address (`127.0.0.1` = this machine only) |
| `PLAID_DATA_DIR` | `/var/lib/plaid` (package), `/data` (Docker) | Database and caches |
| `PLAID_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PLAID_LOG_FORMAT` | `text` (package), `json` (Docker) | `text` or `json` |
| `PLAID_SERVER_NAME` | the machine's hostname | Name the apps show for this server |
| `PLAID_TRUSTED_PROXIES` | empty | Reverse proxies (IPs or CIDRs, comma-separated) whose `X-Forwarded-For` / `X-Forwarded-Proto` headers PLAID believes. See below |
| `PLAID_FFPROBE_PATH` | `ffprobe` on `PATH` | ffprobe used to analyze media files. Without one, files are listed but not analyzed |
| `PLAID_FFMPEG_PATH` | `ffmpeg` on `PATH` | ffmpeg used to convert files for players that can't play them as they are. Without one, only Direct Play works |
| `PLAID_BROWSE_ROOTS` | empty (whole disk) | Folders the library folder picker may show, colon-separated absolute paths, e.g. `/srv/media:/mnt` |
| `PLAID_TMDB_API_KEY` | the key built into release binaries | Your own [TMDB](https://www.themoviedb.org/settings/api) key for posters and synopses. See [Metadata](#metadata-posters-and-synopses) |
| `PLAID_FANART_API_KEY` | the key built into release binaries | Your own [fanart.tv](https://fanart.tv/get-an-api-key/) project key for logos and clearart |
| `PLAID_ARTWORK_CACHE_MAX_MB` | `2048` | Disk cap for resized images (at least 64). Downloaded originals aren't counted: they're kept for offline use |

## First launch: creating the admin account

Open `http://<server>:32500` in a browser. While the server has no account, PLAID asks you to create the admin account; after that, the page is gone for good.

For safety, this only works from your local network (a private or loopback address). If you're setting PLAID up from elsewhere, for example over a VPN that doesn't look local or through a reverse proxy, the page asks for the **setup code** that PLAID prints in its log at startup while it has no account:

```bash
sudo journalctl -u plaid | grep first_run_setup_code   # native package
docker compose logs plaid | grep first_run_setup_code  # Docker
```

The code is the only secret PLAID ever writes to its log. It changes at every restart and stops working once the admin account exists.

## Behind a reverse proxy

If PLAID sits behind nginx, Caddy or Traefik, tell it which proxy to trust, or every request will look like it comes from the proxy:

```bash
PLAID_TRUSTED_PROXIES=127.0.0.1,::1   # proxy on the same machine
PLAID_TRUSTED_PROXIES=172.18.0.0/16   # proxy in a Docker network
```

The proxy must forward the original `Host` header (the default for Caddy and Traefik, `proxy_set_header Host $host;` for nginx) and set `X-Forwarded-For` and `X-Forwarded-Proto`. PLAID checks that the web app's requests come from the address you typed in the browser; a rewritten `Host` makes them fail with `403 csrf`.

Until `PLAID_TRUSTED_PROXIES` is set, first-run setup through a proxy asks for the setup code (see above), even if the proxy runs on the same machine.

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

Check it worked: `sudo -u plaid ls /path/to/media`. When you add a folder to a library, PLAID checks the same thing and answers `folder_unreadable` if it can't list it.

**Docker:** the container's user is uid 1000. Your media must be readable by that uid (or world-readable, `chmod -R o+rX /path/to/media`). Mount it with `:ro`: PLAID doesn't need more.

PLAID only ever needs **read** access to media; it never writes to your library folders.

## How PLAID notices new files

On local disks (ext4, btrfs, XFS, ZFS, a USB drive…) PLAID watches library folders: a file you copy in is scanned once it has stopped growing for 10 seconds. On top of that, every library is rescanned every 6 hours as a safety net.

**Network shares (NFS, SMB/CIFS) and FUSE mounts** don't report changes reliably, so PLAID doesn't watch them. It rescans those libraries every **15 minutes** instead. Use "Scan now" for anything you want sooner. You can change a library's interval (5 minutes to 24 hours) in its settings.

If a drive is unplugged or a share is down, PLAID marks the folder `unavailable` and leaves everything as it was: nothing is removed. Plug it back in and the next scan picks up where it left off. Files that really disappear are hidden from the library, then forgotten after 30 days.

### Big libraries: the inotify watch limit

Linux limits how many folders one user can watch (`fs.inotify.max_user_watches`). Kernels from 5.11 on size it from your RAM, from about 8,000 on 512 MB up to 1,000,000, which is plenty for most libraries. PLAID uses one watch per folder. If it runs out, the affected library folder falls back to periodic scans and PLAID logs a warning. The admin capabilities (`GET /api/v1/server/capabilities`) list the folders concerned.

To raise the limit:

```bash
echo fs.inotify.max_user_watches=524288 | sudo tee /etc/sysctl.d/60-plaid-inotify.conf
sudo sysctl --system
sudo systemctl restart plaid
```

In Docker, the limit is the host's: run these commands on the host. PLAID's package doesn't change this setting for you, as it applies to the whole machine.

## Metadata: posters and synopses

PLAID matches your movies and shows to [TMDB](https://www.themoviedb.org) and adds logos and clearart from [fanart.tv](https://fanart.tv), in each library's language (Settings → Libraries; it defaults to the language of the admin who created the library). Local files win: a Kodi-style `movie.nfo` / `tvshow.nfo` and images such as `poster.jpg`, `fanart.jpg`, `clearlogo.png` or `season01-poster.jpg` next to your media are used first. PLAID only reads them; it never writes in your media folders.

**Keys.** Release binaries come with PLAID's own TMDB and fanart.tv keys, so this works with no setup. You can use your own instead, in the admin settings or with `PLAID_TMDB_API_KEY` / `PLAID_FANART_API_KEY`. The order is: the key saved in the admin settings, then the environment variable, then the built-in key. Keys saved in the admin settings are stored in `plaid.db`, which should only be readable by PLAID (`chmod 600`; the server warns at startup otherwise). A build from source has no built-in key: without one, PLAID uses `.nfo` files and local images only.

**What is sent.** To match and fetch artwork, PLAID sends TMDB and fanart.tv the titles, years and ids of your movies and shows, from your server's IP address. Nothing about your accounts, profiles or what anyone watches is ever sent. The apps never call these services themselves: images go through your server.

**Offline.** Everything already fetched stays available offline. New items show their file names until the providers are reachable again; PLAID retries on its own.

**Disk.** Downloaded originals live in `PLAID_DATA_DIR/artwork` (about 2 GB for 1,000 movies and 100 shows); back them up with `plaid.db`, or they're downloaded again. Resized copies live in `PLAID_DATA_DIR/cache/artwork`, capped by `PLAID_ARTWORK_CACHE_MAX_MB`, and can be deleted at any time.

**Attribution.** This product uses the TMDB API but is not endorsed or certified by TMDB. Logos, clearart and some backdrops come from fanart.tv.

## Playing a movie with curl and VLC

The web player arrives with the web app. Until then, any player that opens a URL can play your movies, and so will external players later on.

1. Sign in and keep the token. If your account has several profiles, pick one with `POST /api/v1/session/profile`:

   ```bash
   TOKEN=$(curl -s http://<server>:32500/api/v1/auth/login -H 'Content-Type: application/json' \
     -d '{"username": "you", "password": "…", "device": {"name": "Terminal", "type": "other"}}' | jq -r .token)
   ```

2. Find the movie's id (`GET /api/v1/libraries`, then `GET /api/v1/libraries/<library id>/items`).

3. Start playing it with the `external` profile, which means "this player plays every file as it is":

   ```bash
   curl -s http://<server>:32500/api/v1/items/<item id>/playback -H "Authorization: Bearer $TOKEN" \
     -H 'Content-Type: application/json' -d '{"profile": {"preset": "external"}}' | jq -r .url
   ```

4. Open `http://<server>:32500` followed by that URL in VLC (Media → Open Network Stream) or `mpv`. Seeking works.

The link carries its own key: anyone who has it can play that movie until the session ends, so don't share it. It stops working when you end it (`DELETE /api/v1/playback/<id>`), 24 hours after it was last used, 7 days after it was made at the latest, or as soon as the device that made it is signed out.

Files a player can't play as they are need FFmpeg and arrive with a later version of PLAID; until then, PLAID answers `ffmpeg_missing` for them.

