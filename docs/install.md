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

The package depends on your distribution's `ffmpeg`, which apt installs with it: PLAID uses `ffprobe` to analyze media files and `ffmpeg` to convert them for players that can't play them as they are.

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

Edit `docker-compose.yml` to mount your media folders read-only (`/path/to/movies:/media/movies:ro`), then use the paths **inside the container** (`/media/movies`) when you create a library. Data lives in the `plaid-data` volume. The container runs as a non-root user (uid 1000) and ships FFmpeg (with the Intel and AMD video drivers on `amd64`). To give it a GPU for transcoding, see [Hardware transcoding](#hardware-transcoding).

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
| `PLAID_TRANSCODE_DIR` | `<data dir>/transcodes` | Temporary video segments while converting. Emptied at every start. See [Where conversions are written](#where-conversions-are-written) |
| `PLAID_BROWSE_ROOTS` | empty (whole disk) | Folders the library folder picker may show, colon-separated absolute paths, e.g. `/srv/media:/mnt` |
| `PLAID_TMDB_API_KEY` | the key built into release binaries | Your own [TMDB](https://www.themoviedb.org/settings/api) key for posters and synopses. See [Metadata](#metadata-posters-and-synopses) |
| `PLAID_FANART_API_KEY` | the key built into release binaries | Your own [fanart.tv](https://fanart.tv/get-an-api-key/) project key for logos and clearart |
| `PLAID_ARTWORK_CACHE_MAX_MB` | `2048` | Disk cap for resized images (at least 64). Downloaded originals aren't counted: they're kept for offline use |

## First launch: creating the admin account

Open `http://<server>:32500` in a browser. While the server has no account, PLAID asks you to create the admin account; after that, the page is gone for good.

For safety, this only works from your local network (a private or loopback address). If you're setting PLAID up from elsewhere, for example over a VPN that doesn't look local or through a reverse proxy, the page asks for the **setup code** that PLAID prints in its log at startup while it has no account:

```bash
sudo journalctl -u plaid | grep first_run_setup_code   # native package
docker logs plaid 2>&1 | grep first_run_setup_code     # Docker
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

Other players say what they can play in the `profile` instead of `external`, and PLAID picks how to send each file: see [Playback](#playback).

## Playback

For each file and player, PLAID picks the lightest way that works and says why in the session's `method` and `reasons`:

| Method | What happens | Cost on the server |
|--------|--------------|--------------------|
| **Direct Play** | The file as it is, over HTTP with ranges (`…/file`). | Almost nothing: no FFmpeg. |
| **Direct Stream** | The video is copied as it is; the container (and the audio, if the player can't decode it) is converted, served as HLS (`…/master.m3u8`). | Light: no video decoding. |
| **Transcode** | The video is re-encoded to H.264 at a quality that fits the player and the connection, served as HLS. | Heavy: one CPU core or more per stream in software, little with a GPU. |

Admins tune it with `GET`/`PATCH /api/v1/settings/playback`: simultaneous conversions (`maxTranscodes`, 1 by default on 4 cores or fewer, 2 above, 4 with a GPU), a bitrate cap for clients outside the home network (`remoteMaxBitrate`), extra networks counted as home (`localNetworks`, e.g. `100.64.0.0/10` for Tailscale), the encoder speed (`encoderPreset`: `fast` by default, one step faster still on 4 cores or fewer, `balanced`, `quality`) and the GPU (`hwAccel`, below). `GET /api/v1/playback` lists who is watching what, how, and how fast conversions run.

### Hardware transcoding

PLAID uses the GPU when it can (`hwAccel: auto`, the default): at start, it tries VAAPI, QSV, NVENC and V4L2 M2M in that order with a 1-second test encode, and keeps the first that works. `GET /api/v1/server/capabilities` shows the result under `hwAccel`. If a conversion fails on the GPU, it restarts in software; after 3 failures in 10 minutes, PLAID stays in software until it restarts or the setting changes (`disabledReason: repeated_failures`).

What works with your distribution's FFmpeg:

| System | Intel / AMD (VAAPI) | Intel (QSV) | NVIDIA (NVENC) | Raspberry Pi (V4L2 M2M) |
|--------|---------------------|-------------|----------------|--------------------------|
| Debian 13, Ubuntu 24.04+ (`amd64`) | ✅ | ✅ | ✅ with the NVIDIA driver | — |
| Debian 12 (`amd64`) | ✅ | ⚠️ older Intel GPUs only | ✅ with the NVIDIA driver | — |
| Raspberry Pi 4 (`arm64`) | — | — | — | ⚠️ H.264 encoding only: software in practice |
| Raspberry Pi 5 (`arm64`) | — | — | — | ❌ no video encoder: software only |
| Docker image | ✅ (`amd64`) | ✅ (`amd64`) | ✅ with the NVIDIA Container Toolkit | — |

To choose one yourself, `PATCH /api/v1/settings/playback` with `{"hwAccel": "vaapi"}` (or `qsv`, `nvenc`, `v4l2m2m`, `none`), and `"hwDevice": "/dev/dri/renderD129"` when the machine has several GPUs (the first `/dev/dri/renderD*` otherwise). PLAID tests it first and answers `422 hw_accel_unavailable` if it doesn't work, without changing anything.

**Native package.** The package adds the `plaid` user to the `video` and `render` groups, so Intel and AMD GPUs work with nothing to do: check that `ls -l /dev/dri` shows `renderD128` owned by the `render` group. Install the VAAPI driver if your system doesn't have it (`intel-media-va-driver` for Intel since 2015, `mesa-va-drivers` for AMD). The service is hardened with `MemoryDenyWriteExecute`, which the NVIDIA driver (and the odd other one) doesn't support. If the GPU test fails with an NVIDIA card, or the logs show `hw_accel_unavailable`, allow it for PLAID only:

```bash
sudo systemctl edit plaid
```

```ini
[Service]
MemoryDenyWriteExecute=false
```

then `sudo systemctl restart plaid`.

**Docker.** Pass the GPU to the container. For Intel and AMD, add the device and the host's `render` group, whose number you get with `getent group render | cut -d: -f3`:

```yaml
    devices:
      - /dev/dri:/dev/dri
    group_add:
      - "993"   # your render group's number
```

For NVIDIA, install the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) on the host and add:

```yaml
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu, video]
```

Both are in `docker-compose.yml`, commented out.

### Where conversions are written

While converting, FFmpeg writes short video segments to `PLAID_TRANSCODE_DIR` (`/var/lib/plaid/transcodes` for the package, `/data/transcodes` in Docker): one folder per playback, removed when it ends, segments more than 5 minutes behind the player deleted as it goes, and everything emptied at start. The folder is capped at 10 GiB, or 20% of the free space when that's less (never under 1 GiB); `transcodeDirMaxBytes` in the playback settings changes it. When it's full, new conversions are refused with `transcode_storage_full`.

A folder in memory saves disk writes, if the machine has RAM to spare (allow about 1 GiB per simultaneous conversion). With the package, the service may only write to `/var/lib/plaid`, so any other folder also needs a `ReadWritePaths=` line. For a 4 GiB folder in memory:

```bash
sudo mkdir -p /mnt/plaid-transcodes
echo "tmpfs /mnt/plaid-transcodes tmpfs size=4G,mode=0750,uid=$(id -u plaid),gid=$(id -g plaid) 0 0" | sudo tee -a /etc/fstab
sudo mount /mnt/plaid-transcodes
sudo systemctl edit plaid
```

```ini
[Service]
ReadWritePaths=/mnt/plaid-transcodes
```

then set `PLAID_TRANSCODE_DIR=/mnt/plaid-transcodes` in `/etc/plaid/plaid.env` and `sudo systemctl restart plaid`. In Docker, the `tmpfs:` example in `docker-compose.yml` does the same.

