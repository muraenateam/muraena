#!/bin/bash
#
# Mitmproxy Setup Script for Muraena Integration
# This script automates the installation and configuration of mitmproxy/mitmweb
# for use with Muraena on Debian 13
#
# Usage: sudo ./setup-mitmproxy.sh
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
MITMPROXY_VERSION="10.1.6"
INSTALL_METHOD="pip"  # Options: "pip" or "binary"
MITMPROXY_DIR="/opt/mitmproxy"
LOG_DIR="/var/log/mitmproxy"
LIB_DIR="/var/lib/mitmproxy"
CONFIG_DIR="/etc/mitmproxy"

# Proxy configuration
PROXY_PORT="8080"
WEB_PORT="8081"

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo_error "This script must be run as root"
        exit 1
    fi
}

check_debian() {
    if [ ! -f /etc/debian_version ]; then
        echo_error "This script is designed for Debian systems"
        exit 1
    fi
    echo_info "Running on Debian $(cat /etc/debian_version)"
}

install_dependencies() {
    echo_info "Installing dependencies..."
    apt update
    apt install -y python3-pip python3-venv python3-dev build-essential \
                   libssl-dev libffi-dev wget curl
}

install_mitmproxy_pip() {
    echo_info "Installing mitmproxy via pip..."

    # Create virtual environment
    mkdir -p "$MITMPROXY_DIR"
    python3 -m venv "$MITMPROXY_DIR/venv"

    # Activate and install
    source "$MITMPROXY_DIR/venv/bin/activate"
    pip install --upgrade pip
    pip install mitmproxy

    # Create wrapper scripts
    cat > /usr/local/bin/mitmproxy <<EOF
#!/bin/bash
source $MITMPROXY_DIR/venv/bin/activate
exec mitmproxy "\$@"
EOF

    cat > /usr/local/bin/mitmweb <<EOF
#!/bin/bash
source $MITMPROXY_DIR/venv/bin/activate
exec mitmweb "\$@"
EOF

    cat > /usr/local/bin/mitmdump <<EOF
#!/bin/bash
source $MITMPROXY_DIR/venv/bin/activate
exec mitmdump "\$@"
EOF

    chmod +x /usr/local/bin/mitmproxy
    chmod +x /usr/local/bin/mitmweb
    chmod +x /usr/local/bin/mitmdump

    echo_info "mitmproxy installed via pip"
}

install_mitmproxy_binary() {
    echo_info "Installing mitmproxy from binary..."

    cd /tmp
    wget "https://snapshots.mitmproxy.org/${MITMPROXY_VERSION}/mitmproxy-${MITMPROXY_VERSION}-linux-x86_64.tar.gz"

    mkdir -p "$MITMPROXY_DIR"
    tar -xzf "mitmproxy-${MITMPROXY_VERSION}-linux-x86_64.tar.gz" -C "$MITMPROXY_DIR/"

    # Create symlinks
    ln -sf "$MITMPROXY_DIR/mitmproxy" /usr/local/bin/mitmproxy
    ln -sf "$MITMPROXY_DIR/mitmweb" /usr/local/bin/mitmweb
    ln -sf "$MITMPROXY_DIR/mitmdump" /usr/local/bin/mitmdump

    rm -f "mitmproxy-${MITMPROXY_VERSION}-linux-x86_64.tar.gz"

    echo_info "mitmproxy installed from binary"
}

create_user() {
    echo_info "Creating mitmproxy user..."

    if ! id -u mitmproxy > /dev/null 2>&1; then
        useradd -r -s /bin/false -d "$LIB_DIR" mitmproxy
        echo_info "User 'mitmproxy' created"
    else
        echo_warn "User 'mitmproxy' already exists"
    fi

    # Create directories
    mkdir -p "$LOG_DIR"
    mkdir -p "$LIB_DIR"
    mkdir -p "$CONFIG_DIR"

    chown -R mitmproxy:mitmproxy "$LOG_DIR"
    chown -R mitmproxy:mitmproxy "$LIB_DIR"
}

create_config() {
    echo_info "Creating configuration file..."

    cat > "$CONFIG_DIR/config.yaml" <<EOF
# mitmproxy configuration for Muraena integration

# Listen on localhost only for security
listen_host: 127.0.0.1
listen_port: $PROXY_PORT

# Web interface configuration
web_host: 0.0.0.0
web_port: $WEB_PORT

# SSL/TLS configuration
# Don't verify upstream SSL (Muraena handles this)
ssl_insecure: true

# Increase flow limits for high-traffic scenarios
flow_detail: 3

# Store flows for later analysis
save_stream_file: $LOG_DIR/flows.mitm

# Upstream proxy mode (transparent)
mode: regular
EOF

    echo_info "Configuration created at $CONFIG_DIR/config.yaml"
}

create_systemd_service() {
    echo_info "Creating systemd service..."

    # Determine ExecStart based on installation method
    if [ "$INSTALL_METHOD" = "pip" ]; then
        EXEC_START="$MITMPROXY_DIR/venv/bin/mitmweb"
    else
        EXEC_START="$MITMPROXY_DIR/mitmweb"
    fi

    cat > /etc/systemd/system/mitmweb.service <<EOF
[Unit]
Description=mitmweb - Web interface for mitmproxy
After=network.target

[Service]
Type=simple
User=mitmproxy
Group=mitmproxy
WorkingDirectory=$LIB_DIR

ExecStart=$EXEC_START \\
    --listen-host 127.0.0.1 \\
    --listen-port $PROXY_PORT \\
    --web-host 0.0.0.0 \\
    --web-port $WEB_PORT \\
    --ssl-insecure \\
    --set flow_detail=3 \\
    --save-stream-file $LOG_DIR/flows.mitm

Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$LOG_DIR $LIB_DIR

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    echo_info "Systemd service created"
}

configure_firewall() {
    echo_info "Configuring firewall (if applicable)..."

    # Check if ufw is installed and active
    if command -v ufw &> /dev/null; then
        if ufw status | grep -q "Status: active"; then
            echo_warn "UFW is active. You may need to allow port $WEB_PORT for remote access"
            echo_warn "Run: sudo ufw allow from YOUR_IP to any port $WEB_PORT"
        fi
    fi

    # Check if iptables has rules
    if iptables -L | grep -q "Chain INPUT"; then
        echo_warn "iptables rules detected. You may need to allow port $WEB_PORT"
        echo_warn "For SSH tunnel access, no firewall changes needed (recommended)"
    fi
}

print_completion_message() {
    echo ""
    echo_info "=========================================="
    echo_info "Mitmproxy installation complete!"
    echo_info "=========================================="
    echo ""
    echo_info "Next steps:"
    echo ""
    echo "1. Start mitmweb service:"
    echo "   sudo systemctl start mitmweb"
    echo ""
    echo "2. Enable mitmweb to start on boot:"
    echo "   sudo systemctl enable mitmweb"
    echo ""
    echo "3. Check service status:"
    echo "   sudo systemctl status mitmweb"
    echo ""
    echo "4. Run Muraena with proxy mode:"
    echo "   ./muraena -config /path/to/config.toml -proxy"
    echo ""
    echo "5. Access mitmweb interface:"
    echo "   - Local: http://localhost:$WEB_PORT"
    echo "   - Remote (via SSH tunnel): ssh -L $WEB_PORT:localhost:$WEB_PORT user@server"
    echo ""
    echo_info "Configuration files:"
    echo "   - Config: $CONFIG_DIR/config.yaml"
    echo "   - Logs: $LOG_DIR/"
    echo "   - Service: /etc/systemd/system/mitmweb.service"
    echo ""
    echo_warn "For detailed documentation, see: docs/MITMPROXY_DEPLOYMENT.md"
    echo ""
}

main() {
    echo_info "Starting mitmproxy setup for Muraena integration..."
    echo ""

    check_root
    check_debian
    install_dependencies

    if [ "$INSTALL_METHOD" = "pip" ]; then
        install_mitmproxy_pip
    else
        install_mitmproxy_binary
    fi

    # Verify installation
    if ! command -v mitmweb &> /dev/null; then
        echo_error "mitmproxy installation failed"
        exit 1
    fi

    echo_info "mitmproxy version: $(mitmproxy --version | head -n 1)"

    create_user
    create_config
    create_systemd_service
    configure_firewall

    print_completion_message
}

# Run main function
main
