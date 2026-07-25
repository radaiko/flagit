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
- 🔒 **Secure by design** — public API via Tailscale Funnel, admin UI on Tailscale network only
- 🇩🇪 **Dual language** — German and English, auto-detected with manual toggle

## Tech Stack

- **Backend:** Go (chi router) + SQLite (modernc.org/sqlite, no CGO)
- **Frontend:** Svelte (embed overlay + dashboard in Go binary)
- **Deployment:** Docker + Docker Compose + Tailscale

## Quick Start

```bash
# Build and run with Docker
docker compose up -d
```

## Documentation

Full requirements and architecture: see [Flagit.md](https://github.com/radaiko/flagit)

## License

MIT