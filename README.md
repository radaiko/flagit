# Flagit

**A self-hosted ticket system for apps — users flag issues from inside the app, Hermes picks them up and dispatches Claude Code to fix them.**

No user accounts, no email, no contact info — just a unique ticket ID for two-way communication.

## Features

- 🎫 **Ticket creation** — bug reports and feature requests from inside any app
- 🔑 **Device ownership token** — only the original reporter can view/reply to their ticket
- 📋 **Anonymous diagnostics** — app version, OS, platform, ring-buffer logs (PII-stripped)
- 💬 **Two-way messaging** — users and devs can converse on tickets
- 🤖 **Hermes integration** — new tickets trigger automated investigation via Claude Code
- 🌐 **Web overlay** — embeddable web UI for apps (webview/modal)
- 📊 **Admin dashboard** — manage tickets, mass operations, per-app settings
- 🔒 **Secure by design** — public API behind TLS on your own domain (or Tailscale Funnel), admin UI on your tailnet only
- 🇩🇪 **Dual language** — German and English, auto-detected with manual toggle

## Tech Stack

- **Backend:** Go (chi router) + SQLite (modernc.org/sqlite, no CGO)
- **Frontend:** Svelte (embed overlay + dashboard in Go binary)
- **Deployment:** Docker + Docker Compose + Tailscale

## Quick Start

```bash
# Build and run with Docker. Neither port is published to the host: the public
# API is meant to sit behind a reverse proxy, and the admin dashboard behind the
# Tailscale sidecar (set TS_AUTHKEY).
docker compose up -d

# Locally, publish both on loopback instead and skip the sidecar — dashboard at
# http://127.0.0.1:3000/internal/admin
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d flagit
```

See [docs/deployment.html](docs/deployment.html) for the Coolify and Tailscale setup.

## Documentation

Full requirements and architecture: see [Flagit.md](https://github.com/radaiko/flagit)

## License

MIT — see [LICENSE](LICENSE).