# Mitmproxy + Muraena Quick Start Guide

This is a condensed quick-start guide for setting up mitmproxy with Muraena. For detailed information, see [MITMPROXY_DEPLOYMENT.md](MITMPROXY_DEPLOYMENT.md).

## Quick Install (Automated)

```bash
# Clone or navigate to Muraena directory
cd /path/to/muraena

# Run automated setup script
sudo ./scripts/setup-mitmproxy.sh

# Start mitmweb
sudo systemctl start mitmweb
sudo systemctl enable mitmweb

# Verify it's running
sudo systemctl status mitmweb
```

## Quick Install (Manual)

```bash
# Install mitmproxy
sudo apt update
sudo apt install -y python3-pip python3-venv
sudo mkdir -p /opt/mitmproxy
sudo python3 -m venv /opt/mitmproxy/venv
source /opt/mitmproxy/venv/bin/activate
pip install mitmproxy

# Start mitmweb
mitmweb --listen-host 127.0.0.1 --listen-port 8080 --web-host 0.0.0.0 --web-port 8081 --ssl-insecure
```

## Start Muraena with Proxy Mode

```bash
# Run Muraena with proxy flag
./muraena -config config/config.toml -proxy

# Or with systemd
sudo systemctl start muraena
```

## Access mitmweb Interface

### Local Access
```bash
# Open in browser
http://localhost:8081
```

### Remote Access (SSH Tunnel - Recommended)
```bash
# From your local machine
ssh -L 8081:localhost:8081 user@your-server-ip

# Then open browser to
http://localhost:8081
```

## Common Commands

```bash
# Service Management
sudo systemctl start mitmweb       # Start mitmweb
sudo systemctl stop mitmweb        # Stop mitmweb
sudo systemctl restart mitmweb     # Restart mitmweb
sudo systemctl status mitmweb      # Check status

# View Logs
sudo journalctl -u mitmweb -f      # Follow mitmweb logs
sudo journalctl -u muraena -f      # Follow Muraena logs

# Test Proxy
curl -x http://127.0.0.1:8080 http://example.com

# Check Ports
sudo netstat -tlnp | grep -E '(8080|8081)'
```

## mitmweb Interface Tips

### Filter Traffic
```
~d example.com              # Filter by domain
~m POST                     # Filter by method
~c 200                      # Filter by status code
~u /login                   # Filter by path
~d example.com & ~m POST    # Combine filters
```

### Keyboard Shortcuts
- `?` - Show help
- `z` - Clear flow list
- `e` - Edit request/response
- `r` - Replay request
- `f` - Set filter
- `q` - Quit / Back

## File Locations

```
Config:     /etc/mitmproxy/config.yaml
Service:    /etc/systemd/system/mitmweb.service
Logs:       /var/log/mitmproxy/
Flows:      /var/log/mitmproxy/flows.mitm
```

## Troubleshooting

### mitmweb not receiving traffic
```bash
# Check mitmproxy is running
sudo systemctl status mitmweb

# Check Muraena is using -proxy flag
ps aux | grep muraena | grep proxy

# Test proxy manually
curl -x http://127.0.0.1:8080 -I https://example.com
```

### Can't access mitmweb interface
```bash
# Check if listening on correct port
sudo netstat -tlnp | grep 8081

# Check firewall
sudo ufw status
```

### High memory usage
```bash
# Clear old flows
sudo rm /var/log/mitmproxy/flows.mitm
sudo systemctl restart mitmweb

# Or limit flow storage
# Edit /etc/systemd/system/mitmweb.service
# Add to ExecStart: --set stream_large_bodies=1m
```

## Architecture Diagram

```
┌────────┐         ┌─────────┐         ┌──────────┐         ┌────────┐
│ Victim │────────>│ Muraena │────────>│ mitmproxy│────────>│ Target │
└────────┘         └─────────┘         └──────────┘         └────────┘
                                             │
                                             ▼
                                        ┌─────────┐
                                        │ mitmweb │ (Inspection)
                                        │ :8081   │
                                        └─────────┘
```

## Security Checklist

- [ ] mitmweb accessible only via SSH tunnel or VPN
- [ ] Strong authentication if using nginx proxy
- [ ] Log files have proper permissions (0640)
- [ ] Regular log rotation configured
- [ ] Firewall rules restricting access to port 8081
- [ ] Monitor disk space for log files

## Example Configuration

### Muraena with mitmproxy
```bash
# Start mitmweb in background
sudo systemctl start mitmweb

# Run Muraena with proxy mode
./muraena -config config/config.toml -proxy
```

### Systemd Service for Muraena
```ini
[Unit]
Description=Muraena with mitmproxy
After=mitmweb.service
Requires=mitmweb.service

[Service]
ExecStart=/opt/muraena/build/muraena -config /opt/muraena/config/config.toml -proxy
Restart=always

[Install]
WantedBy=multi-user.target
```

## Advanced: Custom mitmproxy Script

Create `/etc/mitmproxy/scripts/custom.py`:

```python
from mitmproxy import http
import logging

class CustomLogger:
    def request(self, flow: http.HTTPFlow):
        # Log specific requests
        if "login" in flow.request.url:
            logging.info(f"Login attempt: {flow.request.url}")

    def response(self, flow: http.HTTPFlow):
        # Log specific responses
        if flow.response.status_code >= 400:
            logging.warning(f"Error response: {flow.response.status_code}")

addons = [CustomLogger()]
```

Load the script:
```bash
mitmweb -s /etc/mitmproxy/scripts/custom.py
```

## Resources

- Full Documentation: [MITMPROXY_DEPLOYMENT.md](MITMPROXY_DEPLOYMENT.md)
- mitmproxy Docs: https://docs.mitmproxy.org/
- Muraena Docs: https://muraena.phishing.click/
- Setup Script: `scripts/setup-mitmproxy.sh`

## Support

For issues:
- Check logs: `sudo journalctl -u mitmweb -n 50`
- Verify services: `sudo systemctl status mitmweb muraena`
- Test proxy: `curl -x http://127.0.0.1:8080 http://example.com`

## Quick Reference Card

| Task | Command |
|------|---------|
| Install mitmproxy | `sudo ./scripts/setup-mitmproxy.sh` |
| Start mitmweb | `sudo systemctl start mitmweb` |
| Stop mitmweb | `sudo systemctl stop mitmweb` |
| View logs | `sudo journalctl -u mitmweb -f` |
| Access web UI | `http://localhost:8081` |
| SSH tunnel | `ssh -L 8081:localhost:8081 user@server` |
| Run Muraena with proxy | `./muraena -config config.toml -proxy` |
| Test proxy | `curl -x http://127.0.0.1:8080 http://example.com` |
| Clear flows | `sudo rm /var/log/mitmproxy/flows.mitm` |
| Restart both | `sudo systemctl restart mitmweb muraena` |
