# Flagit

Self-hosted in-app ticket system — users flag issues from inside the app, Hermes dispatches Claude Code to fix them.

## Architecture
- Go backend (chi router) + SQLite (modernc.org/sqlite, no CGO)
- Svelte frontend (web overlay + admin dashboard), embedded in Go binary
- Dual deployment: Docker Compose on Coolify (Hetzner VM)

## Key Commands
- `make dev` — run Go dev server
- `make test` — run all tests
- `make build` — build production binary
- `make docker` — build Docker image

## Code Standards
- 90% line coverage target for both Go backend and Svelte UI
- Go: standard `testing` package + testify
- Svelte: vitest + @testing-library/svelte
- German + English i18n (auto-detect locale, manual toggle)

## API Design
- Public API: `/api/tickets` with ownership token auth (hash stored)
- Internal API: `/internal/*` no public exposure
- Webhook to Hermes on new tickets (exponential backoff, max 3 retries)
- Polling endpoint: `/internal/poll?since=<timestamp>`

## Project Structure
```
flagit/
├── cmd/flagit/main.go        # Entry point
├── internal/
│   ├── api/                   # HTTP handlers (public + internal)
│   ├── db/                    # SQLite schema + migrations
│   ├── model/                 # Data structures
│   ├── service/               # Business logic
│   ├── webhook/               # Hermes webhook sender
│   └── overlay/               # Embedded Svelte files
├── web/                       # Svelte frontend
│   ├── src/
│   │   ├── overlay/           # Web overlay for apps
│   │   ├── dashboard/         # Admin dashboard
│   │   └── lib/               # Shared components
│   └── tests/                 # Vitest tests
├── deploy/tailscale/          # serve.json for the admin-dashboard sidecar
├── Dockerfile
├── docker-compose.yml         # Coolify deployment: no host ports published
├── docker-compose.local.yml   # opt-in overlay, publishes both ports on loopback
└── README.md
```

## Deployment
- Coolify (Docker Compose) on a Hetzner VM; public API at https://flagit.spitzbub.app
- Neither port is published to the host: Coolify's proxy routes the domain to container port 8080
- Admin dashboard (3000) is reachable only through the Tailscale sidecar `flagit-admin`; it never gets a public domain
- Required secrets, set in Coolify and never in the repo: `FLAGIT_ADMIN_KEY`, `TS_AUTHKEY`
- SQLite persists on the `flagit_data` volume at `/data`; tailnet identity on `tailscale_state`
- Deployed revision: compose maps Coolify's `SOURCE_COMMIT` onto `FLAGIT_COMMIT` at runtime — no
  Coolify setting needed. Runtime, not a build arg: Coolify keeps `SOURCE_COMMIT` out of builds by
  default, and Compose build packs pass build vars via `--env-file`. `Dockerfile`'s `ARG GIT_COMMIT`
  (→ `-X flagit/internal/version.Commit`) is the fallback for `make build`/`make docker`; runtime
  wins, then the build value, then `unknown`

## Key Features
- Ticket creation (bug/feature) with unique ID (e.g. FLG-7X3K9Q)
- Device ownership token (UUID) for unauthenticated auth
- Anonymous diagnostic info (app version, OS, ring buffer logs)
- Two-way messaging (user ↔ agent) per ticket
- Status workflow: open → in-progress → resolved → shipped → closed
- Per-app auto-process toggle (default: off)
- Global auto-process toggle for new unknown apps (default: off)
- Mass operations (bulk status update e.g. mark shipped in version X)
- Commit history (dev-only, admin dashboard only)
- Deployed commit in the dashboard footer (short SHA, full SHA on hover + copy), served by
  `GET /internal/version` behind the admin key — never on the public API