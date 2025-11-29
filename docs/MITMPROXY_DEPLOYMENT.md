# Mitmproxy + Muraena Deployment Guide

This guide explains how to deploy and configure mitmproxy/mitmweb alongside Muraena on a Debian 13 instance. This setup allows you to inspect all HTTP/HTTPS traffic flowing through Muraena using mitmweb's web interface.

## Architecture Overview

```
Victim → Muraena → mitmproxy → Target Server
                      ↓
                   mitmweb (inspection interface)
```

Muraena uses the `-proxy` flag to route all upstream traffic through mitmproxy, which acts as a transparent proxy while providing full traffic visibility through mitmweb.

## Prerequisites

- Debian 13 instance with Muraena already installed
- Root or sudo access
- Python 3.9 or higher (comes with Debian 13)
- Sufficient disk space for traffic logs

## Installation

### 1. Install mitmproxy

#### Option A: Using pip (Recommended)

```bash
# Update system packages
sudo apt update && sudo apt upgrade -y

# Install Python pip and dependencies
sudo apt install -y python3-pip python3-venv python3-dev build-essential libssl-dev libffi-dev

# Create a virtual environment for mitmproxy
sudo mkdir -p /opt/mitmproxy
sudo python3 -m venv /opt/mitmproxy/venv

# Activate the virtual environment
source /opt/mitmproxy/venv/bin/activate

# Install mitmproxy
pip install --upgrade pip
pip install mitmproxy

# Verify installation
mitmproxy --version
mitmweb --version
```

#### Option B: Using official binaries

```bash
# Download the latest release
cd /tmp
wget https://snapshots.mitmproxy.org/10.1.6/mitmproxy-10.1.6-linux-x86_64.tar.gz

# Extract to /opt
sudo mkdir -p /opt/mitmproxy
sudo tar -xzf mitmproxy-10.1.6-linux-x86_64.tar.gz -C /opt/mitmproxy/

# Create symlinks
sudo ln -s /opt/mitmproxy/mitmproxy /usr/local/bin/mitmproxy
sudo ln -s /opt/mitmproxy/mitmweb /usr/local/bin/mitmweb
sudo ln -s /opt/mitmproxy/mitmdump /usr/local/bin/mitmdump

# Verify installation
mitmproxy --version
```

### 2. Create mitmproxy User (Optional but Recommended)

```bash
# Create dedicated user for mitmproxy
sudo useradd -r -s /bin/false -d /var/lib/mitmproxy mitmproxy

# Create directories
sudo mkdir -p /var/lib/mitmproxy
sudo mkdir -p /var/log/mitmproxy
sudo chown -R mitmproxy:mitmproxy /var/lib/mitmproxy
sudo chown -R mitmproxy:mitmproxy /var/log/mitmproxy
```

## Configuration

### 1. Configure mitmproxy

Create a configuration file at `/etc/mitmproxy/config.yaml`:

```bash
sudo mkdir -p /etc/mitmproxy
sudo tee /etc/mitmproxy/config.yaml > /dev/null <<'EOF'
# mitmproxy configuration for Muraena integration

# Listen on localhost only for security
listen_host: 127.0.0.1
listen_port: 8080

# Web interface configuration
web_host: 0.0.0.0
web_port: 8081

# SSL/TLS configuration
# Don't verify upstream SSL (Muraena handles this)
ssl_insecure: true

# Increase flow limits for high-traffic scenarios
flow_detail: 3

# Store flows for later analysis
save_stream_file: /var/log/mitmproxy/flows.mitm

# Upstream proxy mode (transparent)
mode: regular

# Optional: Set upstream proxy if needed
# upstream_cert: false
EOF
```

### 2. Create systemd Service for mitmweb

Create `/etc/systemd/system/mitmweb.service`:

```bash
sudo tee /etc/systemd/system/mitmweb.service > /dev/null <<'EOF'
[Unit]
Description=mitmweb - Web interface for mitmproxy
After=network.target

[Service]
Type=simple
User=mitmproxy
Group=mitmproxy
WorkingDirectory=/var/lib/mitmproxy

# If using pip installation
ExecStart=/opt/mitmproxy/venv/bin/mitmweb \
    --listen-host 127.0.0.1 \
    --listen-port 8080 \
    --web-host 0.0.0.0 \
    --web-port 8081 \
    --ssl-insecure \
    --set flow_detail=3 \
    --save-stream-file /var/log/mitmproxy/flows.mitm

# If using binary installation, use this instead:
# ExecStart=/opt/mitmproxy/mitmweb \
#     --listen-host 127.0.0.1 \
#     --listen-port 8080 \
#     --web-host 0.0.0.0 \
#     --web-port 8081 \
#     --ssl-insecure \
#     --set flow_detail=3 \
#     --save-stream-file /var/log/mitmproxy/flows.mitm

Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/mitmproxy /var/lib/mitmproxy

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

### 3. Enable and Start mitmweb

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable mitmweb to start on boot
sudo systemctl enable mitmweb

# Start mitmweb
sudo systemctl start mitmweb

# Check status
sudo systemctl status mitmweb

# View logs
sudo journalctl -u mitmweb -f
```

## Configure Muraena to Use mitmproxy

### 1. Update Muraena Startup Script

When running Muraena, add the `-proxy` flag to route traffic through mitmproxy:

```bash
# Basic usage
./muraena -config /path/to/config.toml -proxy

# The -proxy flag enables internal proxy mode, which routes traffic through
# an HTTP proxy at http://127.0.0.1:8080 (mitmproxy)
```

### 2. Create systemd Service for Muraena with Proxy

Update or create `/etc/systemd/system/muraena.service`:

```bash
sudo tee /etc/systemd/system/muraena.service > /dev/null <<'EOF'
[Unit]
Description=Muraena Reverse Proxy with mitmproxy integration
After=network.target mitmweb.service
Requires=mitmweb.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/muraena

# Start Muraena with proxy mode enabled
ExecStart=/opt/muraena/build/muraena -config /opt/muraena/config/config.toml -proxy

Restart=always
RestartSec=10

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

### 3. Start Muraena

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable Muraena to start on boot
sudo systemctl enable muraena

# Start Muraena (this will automatically use mitmproxy)
sudo systemctl start muraena

# Check status
sudo systemctl status muraena

# View logs
sudo journalctl -u muraena -f
```

## Accessing mitmweb Interface

### 1. Local Access

If you're on the server locally:

```bash
# Open browser to
http://localhost:8081
```

### 2. Remote Access via SSH Tunnel (Recommended)

For security, expose mitmweb only via SSH tunnel:

```bash
# From your local machine
ssh -L 8081:localhost:8081 user@your-server-ip

# Then open browser to
http://localhost:8081
```

### 3. Remote Access with Firewall (Less Secure)

If you need direct remote access, configure firewall:

```bash
# Allow mitmweb port (be careful with this)
sudo ufw allow from YOUR_IP_ADDRESS to any port 8081

# Or restrict to specific IP
sudo iptables -A INPUT -p tcp -s YOUR_IP_ADDRESS --dport 8081 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8081 -j DROP
```

### 4. Remote Access via Nginx Reverse Proxy (Most Secure)

Create `/etc/nginx/sites-available/mitmweb`:

```nginx
server {
    listen 443 ssl http2;
    server_name mitmweb.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # Basic authentication
    auth_basic "Restricted Access";
    auth_basic_user_file /etc/nginx/.htpasswd;

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

Enable and create password:

```bash
# Create password file
sudo apt install apache2-utils
sudo htpasswd -c /etc/nginx/.htpasswd admin

# Enable site
sudo ln -s /etc/nginx/sites-available/mitmweb /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## Using mitmweb

### Interface Overview

Once connected to mitmweb, you'll see:

1. **Flow List**: All HTTP/HTTPS requests and responses
2. **Flow Details**: Click any flow to see:
   - Request headers and body
   - Response headers and body
   - Timing information
   - WebSocket messages (if applicable)

### Filtering Traffic

Use the filter bar to focus on specific traffic:

```
# Filter by domain
~d example.com

# Filter by method
~m POST

# Filter by status code
~c 200

# Filter by path
~u /login

# Combine filters
~d example.com & ~m POST
```

### Saving and Exporting Flows

```bash
# Flows are automatically saved to /var/log/mitmproxy/flows.mitm

# Export to HAR format
mitmproxy --save-stream-file /var/log/mitmproxy/flows.mitm --set hardump=/var/log/mitmproxy/export.har

# Replay flows
mitmdump --server-replay /var/log/mitmproxy/flows.mitm
```

## Troubleshooting

### Check Service Status

```bash
# Check mitmweb
sudo systemctl status mitmweb
sudo journalctl -u mitmweb -n 50

# Check Muraena
sudo systemctl status muraena
sudo journalctl -u muraena -n 50
```

### Common Issues

#### 1. mitmproxy not receiving traffic from Muraena

```bash
# Verify mitmproxy is listening
sudo netstat -tlnp | grep 8080

# Check Muraena is running with -proxy flag
ps aux | grep muraena

# Test proxy manually
curl -x http://127.0.0.1:8080 http://example.com
```

#### 2. SSL/TLS errors

mitmproxy will intercept SSL connections. Ensure `ssl_insecure: true` is set in config:

```bash
# Update mitmweb service to add --ssl-insecure
sudo systemctl edit mitmweb
```

#### 3. Cannot access mitmweb interface

```bash
# Check if mitmweb is binding correctly
sudo netstat -tlnp | grep 8081

# Check firewall
sudo ufw status
sudo iptables -L -n | grep 8081

# Check logs
sudo journalctl -u mitmweb -f
```

#### 4. High memory usage

For high-traffic scenarios:

```bash
# Limit flow retention in mitmweb
# Edit service file
sudo systemctl edit mitmweb

# Add to ExecStart:
--set stream_large_bodies=1m

# Or disable flow storage temporarily
--no-web-open-browser --set stream_large_bodies=1m
```

### Performance Tuning

For production environments with high traffic:

```bash
# Increase file limits
sudo tee -a /etc/security/limits.conf > /dev/null <<'EOF'
mitmproxy soft nofile 65536
mitmproxy hard nofile 65536
EOF

# Update systemd service
sudo systemctl edit mitmweb

# Add:
[Service]
LimitNOFILE=65536
```

## Security Considerations

### 1. Restrict mitmweb Access

- **Never** expose mitmweb directly to the internet
- Use SSH tunnels or VPN for remote access
- If using nginx proxy, enforce strong authentication
- Consider IP whitelisting

### 2. Protect Logged Traffic

```bash
# Encrypt logs at rest
sudo apt install ecryptfs-utils
sudo mount -t ecryptfs /var/log/mitmproxy /var/log/mitmproxy

# Rotate logs regularly
sudo tee /etc/logrotate.d/mitmproxy > /dev/null <<'EOF'
/var/log/mitmproxy/*.mitm {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 mitmproxy mitmproxy
}
EOF
```

### 3. Audit Access

```bash
# Enable audit logging for mitmweb access
sudo apt install auditd
sudo auditctl -w /var/log/mitmproxy -p rwxa -k mitmproxy_access
```

## Advanced Configuration

### Custom mitmproxy Scripts

Create custom scripts for advanced traffic manipulation:

```python
# /etc/mitmproxy/scripts/logger.py
from mitmproxy import http

def request(flow: http.HTTPFlow) -> None:
    # Log all requests to custom file
    with open("/var/log/mitmproxy/custom.log", "a") as f:
        f.write(f"{flow.request.method} {flow.request.url}\n")

def response(flow: http.HTTPFlow) -> None:
    # Log response status
    with open("/var/log/mitmproxy/custom.log", "a") as f:
        f.write(f"Response: {flow.response.status_code}\n")
```

Load script in service:

```bash
sudo systemctl edit mitmweb

# Add to ExecStart:
-s /etc/mitmproxy/scripts/logger.py
```

### Monitoring and Alerts

Set up monitoring for suspicious activity:

```bash
# Install monitoring tools
sudo apt install prometheus-node-exporter

# Monitor mitmproxy metrics
# Add to Prometheus config
```

## Backup and Recovery

### Backup Flows

```bash
# Create backup script
sudo tee /usr/local/bin/backup-mitmproxy.sh > /dev/null <<'EOF'
#!/bin/bash
BACKUP_DIR="/backup/mitmproxy"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR
tar -czf $BACKUP_DIR/flows_$DATE.tar.gz /var/log/mitmproxy/*.mitm
find $BACKUP_DIR -type f -mtime +30 -delete
EOF

sudo chmod +x /usr/local/bin/backup-mitmproxy.sh

# Add to crontab
sudo crontab -e
# Add: 0 2 * * * /usr/local/bin/backup-mitmproxy.sh
```

## Useful Commands Reference

```bash
# Start/Stop Services
sudo systemctl start mitmweb
sudo systemctl stop mitmweb
sudo systemctl restart mitmweb

sudo systemctl start muraena
sudo systemctl stop muraena
sudo systemctl restart muraena

# View Real-time Logs
sudo journalctl -u mitmweb -f
sudo journalctl -u muraena -f

# Check Listening Ports
sudo netstat -tlnp | grep -E '(8080|8081)'
sudo ss -tlnp | grep -E '(8080|8081)'

# Test Proxy Connection
curl -x http://127.0.0.1:8080 -I https://example.com

# View Saved Flows
mitmdump -r /var/log/mitmproxy/flows.mitm

# Clear Old Flows
sudo rm /var/log/mitmproxy/flows.mitm
sudo systemctl restart mitmweb

# Check Resource Usage
htop -p $(pgrep -f mitmweb)
htop -p $(pgrep -f muraena)
```

## References

- [mitmproxy Documentation](https://docs.mitmproxy.org/stable/)
- [mitmweb User Guide](https://docs.mitmproxy.org/stable/tools-mitmweb/)
- [Muraena GitHub](https://github.com/muraenateam/muraena)
- [mitmproxy Scripting](https://docs.mitmproxy.org/stable/addons-overview/)

## Support

For issues specific to:
- **mitmproxy**: https://github.com/mitmproxy/mitmproxy/issues
- **Muraena**: https://github.com/muraenateam/muraena/issues
