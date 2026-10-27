# PLAID feature backlog

Ideas waiting for an `intent.md`. Nothing here is committed to: pick an item, run `new-intent`, and move it to [roadmap.md](roadmap.md) once it has a feature ID.

**Where these come from:** features the community keeps asking for on the Plex, Jellyfin and Emby feature request forums, plus what people install next to their media server today (Overseerr, Tautulli, Tracearr, Kometa, Bazarr).

**The pitch:** everything you install next to Plex is already built in. Fully self-hosted, no cloud account, free.

## Core gaps in Plex, Jellyfin and Emby

| # | Feature | Why |
|---|---------|-----|
| 1 | No cloud account: everything works offline, no central dependency | Plex depends on plex.tv |
| 2 | Free remote streaming | Plex put it behind a paywall in 2025 |
| 3 | Native SSO / LDAP | Top Emby request, plugin-only on Jellyfin |
| 4 | User groups, per-user permissions and simultaneous stream limits | Emby request since 2014 |
| 5 | Netflix-style watchlist | Highly voted on Jellyfin |
| 6 | Watch Together on every client | Removed from new Plex apps, missing on Jellyfin Android TV. Covered by [F010 Plaid Party](features/F010-plaid-party/intent.md) |
| 7 | HDR transcoding with proper tone mapping | Long-running Jellyfin thread |
| 8 | Smart playlists, "forget a series" in Next Up | Emby |
| 9 | Auto-delete watched items after N days, ignore files during scans | Emby |
| 10 | Adjustable subtitle delay in the player | Emby |
| 11 | Reliable offline downloads for series and playlists | Plex |
| 12 | Full management from TV apps, not web only | Plex |

## Built in instead of a side tool

| # | Feature |
|---|---------|
| 13 | Media requests (Overseerr style) wired to Sonarr and Radarr |
| 14 | Stats and watch history (Tautulli style) |
| 15 | Automatic collections and poster overlays (Kometa style) |
| 16 | Pre-rolls and cinema mode |
| 17 | Native Trakt sync |
| 18 | Import users, watch history and watched state from Plex, Jellyfin and Emby |
| 19 | Multiple versions of a title (4K, 1080p), picked automatically per device |
| 20 | Per-user bandwidth caps |
| 21 | Subtitle download and auto-sync (Bazarr style) |
| 22 | Kids profile with a simpler UI |
| 23 | Extras and featurettes folders for series |
| 24 | Playlist import, hide empty folders |

## Ecosystem and platform

| # | Feature |
|---|---------|
| 25 | Jellyfin and Subsonic API compatibility, so Infuse, Swiftfin, Finamp and Symfonium work on day one. Most strategic item |
| 26 | Remote access without port forwarding (built-in WireGuard tunnel), see [ADR 0003](decisions/0003-remote-access-v1.md) |
| 27 | Audiobooks and podcasts (Plex dropped podcasts) |
| 28 | Music: synced lyrics, smart mixes |
| 29 | Live TV and IPTV from m3u, merged tuners, no duplicate channels |
| 30 | Native intro, credits and recap skip, plus trickplay thumbnails |
| 31 | Chromecast, AirPlay and DLNA that just work |
| 32 | 2FA and an activity log of who watched what, from where |
| 33 | Per-user home screen, hideable libraries |
| 34 | Notifications for new content |
| 35 | One-click backup and restore |
| 36 | Bulk metadata editing, NFO read and write |

## Library and playback comfort

| # | Feature |
|---|---------|
| 37 | Storage health: disk alerts, corrupt file and duplicate detection |
| 38 | Library health: missing metadata, bad naming, auto rename |
| 39 | Offline playback on desktop too |
| 40 | Parental controls by rating, time windows and screen time |
| 41 | Expiring share link for a single title |
| 42 | Nightly pre-transcode of mobile versions |
| 43 | Default audio and subtitle language per user, remembered per series |
| 44 | Personal ratings, "dropped" status, reviews between friends |
| 45 | Calendar of upcoming episodes |
| 46 | Several servers in one app, quick switch |
| 47 | Night mode: volume normalization, dialogue boost |
| 48 | Home videos and photos sorted by date |
| 49 | Mark a whole season watched, "watched up to here" |
| 50 | Playback speed 0.5x to 2x on every client |
| 51 | Dual subtitles (two languages at once) |
| 52 | Subtitle style per device: size, color, position |
| 53 | Advanced search: actor, director, studio, decade, resolution, codec |
| 54 | Actor pages with what you own and what you are missing |
| 55 | Missing episodes in a series, missing films in a collection |
| 56 | Multiple editions: Director's Cut, Extended |
| 57 | External audio tracks and commentaries |
| 58 | Server sleep and Wake-on-LAN from the app |
| 59 | One-click server update from the UI |
| 60 | Series theme songs and video backdrops |

## Profiles and accessibility

| # | Feature |
|---|---------|
| 61 | PIN-protected profiles |
| 62 | Netflix-style profile picker on TV |
| 63 | Guest access without an account, with a watch-time quota |
| 64 | Automatic forced subtitles, SDH filtering |
| 65 | Accessibility: audio description, screen reader, high contrast |
| 66 | Handoff between devices (phone to TV at the same frame) |
| 67 | Editable play queue |
| 68 | Smart shuffle across a series |
| 69 | Sleep timer for video |
| 70 | Delete and move files from the UI |
| 71 | Merge the same title found in several folders |
| 72 | Before playback, show whether it will transcode and why |

## Admin

| # | Feature |
|---|---------|
| 73 | Live dashboard: active streams, CPU/GPU, bandwidth, transcode reason, stop a stream with a message |
| 74 | Config as code (YAML/env), reproducible deploys |
| 75 | Full admin API and CLI (`plaid users add`) |
| 76 | Prometheus metrics and healthcheck for Grafana |
| 77 | Filterable logs in the UI, one-click debug bundle |
| 78 | Visible task scheduler (scans, thumbnails) with off-peak windows |
| 79 | Remote transcode nodes on other machines or GPUs |
| 80 | Multiple storage backends (SMB/NFS, S3, rclone) without needless disk spin-up |
| 81 | Invites with expiry, preset roles, temporary user suspension |
| 82 | Global bandwidth rules by schedule |
| 83 | Admin alerts: disk full, failed scan, failed transcode, new device |
| 84 | Maintenance mode, update rollback, safe DB migrations |

## Monitoring and anti-sharing (Tracearr style)

Built into the server, so Plaid can act instantly instead of polling from outside. Reference: [Tracearr](https://github.com/connorgallopo/Tracearr).

| # | Feature |
|---|---------|
| 85 | Full session history with geolocation (country, city, ASN) on a self-hosted map |
| 86 | Account sharing detection: impossible travel, simultaneous locations, too many devices |
| 87 | Trust score per user, lowered by violations |
| 88 | Automations engine: trigger, conditions, actions (notify, message the client, stop the stream), shareable templates |
| 89 | Built-in rules: no 4K transcode, stop long-paused streams, geo restrictions, inactive accounts |
| 90 | Library analytics: never-watched titles, watch hours per GB, storage forecast |
| 91 | Watch analytics: completion rate, binge detection, viewing by hour and month, genres |
| 92 | Quality stats: 4K/1080p ratio, codecs, device compatibility |
| 93 | Scheduled email newsletter of what was added |
| 94 | Notification destinations: WhatsApp, Discord, ntfy, Gotify, Apprise, Pushover, SMTP, webhook |
| 95 | Import history from Tautulli, Jellystat and Playback Reporting |
| 96 | Bulk actions on users, sessions and violations |

## Requests (Overseerr style)

Built into every Plaid app, so a request can be made straight from the TV.

| # | Feature |
|---|---------|
| 97 | Discover page: trending, popular, upcoming, with an "already on the server" badge |
| 98 | Request a movie, a whole series or specific seasons |
| 99 | Approval workflow: pending, approved, declined with a reason, auto-approve per user or role |
| 100 | Request quotas per user (X movies or seasons per week) |
| 101 | Separate 4K requests with their own rules |
| 102 | Sonarr and Radarr: quality profile, root folder and tags per request |
| 103 | Live status: requested, downloading with progress, available |
| 104 | Notify the requester when it is available |
| 105 | Report an issue on a title (audio, subtitles, wrong file), admin replies |
| 106 | Adding to the watchlist creates a request |
| 107 | Blocklist of titles that cannot be requested |
| 108 | Users vote on requests to prioritize them |
