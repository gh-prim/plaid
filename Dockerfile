# syntax=docker/dockerfile:1

# --- Web app (built once, on the build machine's architecture) ---
FROM --platform=$BUILDPLATFORM node:24-slim AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY api/ /src/api/
COPY web/ ./
RUN pnpm build

# --- Server (cross-compiled, no CGO) ---
FROM --platform=$BUILDPLATFORM golang:1.27 AS server
WORKDIR /src/server
COPY server/ ./
COPY --from=web /src/web/dist/ ./internal/webui/dist/
ARG VERSION=0.1.0-dev
ARG COMMIT=unknown
# No metadata keys here while the image isn't published: set PLAID_TMDB_API_KEY
# at run time. When publishing starts, pass them as build secrets, never ARGs,
# which stay in the image history (ADR 0007).
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w -X github.com/gh-prim/plaid/server/internal/buildinfo.Version=${VERSION} -X github.com/gh-prim/plaid/server/internal/buildinfo.Commit=${COMMIT}" \
      -o /out/plaid ./cmd/plaid

# --- Runtime: Debian slim with the distro FFmpeg (ADR 0006): ffprobe analyzes
# media (F004), ffmpeg transcodes (F006). On amd64, the VAAPI drivers for
# Intel (iHD) and AMD (radeonsi) GPUs; the GPU itself is passed at run time
# (docker-compose.yml, docs/install.md) ---
FROM debian:trixie-slim
ARG TARGETARCH
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata ffmpeg \
 && if [ "$TARGETARCH" = amd64 ]; then \
      apt-get install -y --no-install-recommends intel-media-va-driver mesa-va-drivers; \
    fi \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --system --gid 1000 plaid \
 && useradd --system --uid 1000 --gid plaid --home-dir /data --shell /usr/sbin/nologin plaid \
 && mkdir -p /data/transcodes /media \
 && chown -R plaid:plaid /data
COPY --from=server /out/plaid /usr/bin/plaid

ENV PLAID_DATA_DIR=/data \
    PLAID_LOG_FORMAT=json
EXPOSE 32500 32501
VOLUME ["/data"]
USER plaid
ENTRYPOINT ["/usr/bin/plaid"]
