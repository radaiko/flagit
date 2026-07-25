# syntax=docker/dockerfile:1

# ---------------------------------------------------------------- frontend --
# Builds the Svelte overlay and admin dashboard into web/dist.
FROM node:22-alpine AS web

WORKDIR /build/web

# Dependencies are copied on their own so a source-only change does not
# invalidate the npm layer.
COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# --------------------------------------------------------------- Go binary --
# Pinned to the toolchain in go.mod (1.26). An older image cannot compile it.
FROM golang:1.26-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

# //go:embed reads internal/overlay/dist, so the frontend has to be staged
# there before the compile. The directory is emptied first so a stale
# placeholder or a locally built copy cannot end up in the image.
RUN rm -rf ./internal/overlay/dist && mkdir -p ./internal/overlay/dist
COPY --from=web /build/web/dist/ ./internal/overlay/dist/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /flagit ./cmd/flagit

# ----------------------------------------------------------------- runtime --
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 flagit \
    && mkdir -p /data \
    && chown flagit:flagit /data

COPY --from=build /flagit /flagit

USER flagit
VOLUME /data

# 8080: public API and web overlay. 3000: internal API and admin dashboard.
EXPOSE 8080 3000

# /healthz is unauthenticated on both routers precisely so this works.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

CMD ["/flagit", "--db-path", "/data/flagit.db"]
