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

# -------------------------------------------------------------- provenance --
# Works out which revision this image is being built from, so the admin
# dashboard can name it without anything being configured by hand anywhere.
#
# Its own stage for a reason: the answer lands in a one-line file, and the Go
# build below depends on that file's *content*. Rebuilding the same commit
# therefore still hits the build cache, while a new commit invalidates exactly
# one layer.
FROM alpine:3.19 AS commit

# Both optional and both explicit, highest priority in that order. Unset — which
# is what a raw Docker Compose deployment on Coolify actually produces — the
# resolver falls back to the .git metadata in the build context.
ARG GIT_COMMIT=""
ARG SOURCE_COMMIT=""

COPY scripts/resolve-commit.sh /resolve-commit.sh

# A bind mount of the build context rather than COPY .git: .git may legitimately
# be absent (a tarball build), and COPY of a missing path fails the build, while
# a missing revision must only mean "unknown". .dockerignore keeps everything
# but HEAD, refs and packed-refs out of the context.
RUN --mount=type=bind,target=/context,ro \
    sh /resolve-commit.sh /context > /commit

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

# The revision from the stage above, baked into the binary so the image names
# itself even with no runtime environment at all. A runtime FLAGIT_COMMIT still
# wins over it. Empty — no build argument, no .git in the context — and the
# dashboard reports "unknown".
#
# Copied in as a file rather than taken as a build argument on purpose: only its
# content is in this layer's cache key, and the resolver guarantees it holds a
# hex object name or nothing, so it is safe to substitute into -ldflags.
COPY --from=commit /commit /commit

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X flagit/internal/version.Commit=$(cat /commit)" \
    -o /flagit ./cmd/flagit

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
