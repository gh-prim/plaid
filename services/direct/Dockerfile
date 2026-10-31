# getplaid.direct: one static binary on distroless, running as non-root.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/direct ./cmd/direct \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/direct /direct
# A named volume mounted here starts with this ownership.
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
# Unprivileged ports inside the container: no capability needed. Publish the
# host's 53/udp and 53/tcp to 5353 (see docker-compose.yml).
ENV DIRECT_HTTP_ADDR=:8080 \
    DIRECT_DNS_ADDR=:5353 \
    DIRECT_DATA_DIR=/data
EXPOSE 8080 5353/udp 5353/tcp
VOLUME /data
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/direct", "-healthcheck"]
ENTRYPOINT ["/direct"]
