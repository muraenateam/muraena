# Integration Tests

## Why HTTPS Is Required

When the target uses HTTPS (e.g. `authenticationtest.com`), the proxy **must** also use HTTPS. Running the proxy over plain HTTP against an HTTPS target causes protocol mismatches that hide real bugs and produce false test results:

- **Cookie `Secure` flag**: Browsers only send `Secure` cookies over HTTPS. An HTTP proxy never receives them, so cookie harvesting appears broken even when the code is correct.
- **HSTS**: Browsers enforce HTTPS-only for targets that send `Strict-Transport-Security` headers. An HTTP proxy triggers browser-level blocks.
- **Mixed-content**: Browsers block or warn on HTTP resources loaded from HTTPS pages, changing page behavior.
- **Production detection**: Target sites and WAFs detect protocol downgrades as anomalous.

The integration tests use HTTPS with locally-trusted certificates to match production conditions.

## Prerequisites

1. **Redis** running on `localhost:6379`
2. **dnsmasq** resolving `*.muraena.anti` to `127.0.0.1`
3. **mkcert** installed and initialized (`mkcert -install`)
4. Environment variable: `MURAENA_INTEGRATION=1`
5. Network access to `authenticationtest.com`

## dnsmasq Setup (one-time)

dnsmasq provides wildcard DNS resolution for the `.muraena.anti` local TLD, which `/etc/hosts` cannot do.

```bash
# Install
sudo apt install dnsmasq       # Debian/Ubuntu
# or
sudo pacman -S dnsmasq         # Arch

# Configure
echo 'address=/muraena.anti/127.0.0.1' | sudo tee /etc/dnsmasq.d/muraena.conf

# If systemd-resolved conflicts (port 53 already in use):
# Option A: Configure resolved to forward .muraena.anti queries to dnsmasq
# Option B: Disable the systemd-resolved stub listener:
#   Edit /etc/systemd/resolved.conf, set DNSStubListener=no
#   sudo systemctl restart systemd-resolved

sudo systemctl restart dnsmasq

# Verify
dig evil-authtest.muraena.anti @127.0.0.1
# Should return 127.0.0.1
```

## mkcert Setup (one-time)

mkcert creates locally-trusted development certificates. Its root CA is installed into the system trust store, so no separate `root` PEM is needed in the muraena TLS config.

```bash
# Install (see https://github.com/FiloSottile/mkcert#installation)
go install filippo.io/mkcert@latest

# Initialize local CA (installs root cert into system trust store)
mkcert -install

# Generate wildcard cert for tests
mkdir -p test/integration/certs
cd test/integration/certs
mkcert "*.muraena.anti" "muraena.anti"
# Creates: _wildcard.muraena.anti+1.pem and _wildcard.muraena.anti+1-key.pem
```

The `run_test.sh` script auto-generates certificates if they are missing (requires mkcert to be installed).

## Running Tests

### Automatic (recommended)

```bash
bash test/integration/run_test.sh
```

This script checks all prerequisites, generates certificates if needed, builds muraena, starts it, runs the tests, and cleans up.

### Manual

```bash
export MURAENA_INTEGRATION=1
go test -v -count=1 ./test/integration/
```

## Necrobrowser Integration Testing

To test the necrobrowser integration (post-phishing automation):

1. Install necrobrowser: `cd /path/to/necrobrowser && npm install`
2. Start necrobrowser: `node necrobrowser.js`
3. In `config.toml`, set `[necrobrowser] enable = true` and configure `endpoint` to point to necrobrowser's `/instrument` URL
4. The `instrument.necro` template defines the JSON payload sent to necrobrowser, with placeholders (`%%%TRACKER%%%`, `%%%COOKIES%%%`, `%%%USERAGENT%%%`) replaced at runtime

**Same-IP requirement**: In production, Muraena and Necrobrowser MUST share the same exit IP (same machine, or VPN tunnel). Target sites check for IP consistency across sessions; IP mismatch between the victim's session and necrobrowser's replay is a common anti-hijacking detection.

## Production Deployment Note

- Always use HTTPS when the target uses HTTPS
- In production, obtain real TLS certificates (e.g., Let's Encrypt, Caddy auto-TLS)
- Muraena and Necrobrowser MUST share the same exit IP

## Test Files

| File | Description |
|------|-------------|
| `config.toml` | TOML config targeting `authenticationtest.com` over HTTPS |
| `certs/` | mkcert wildcard certificates (gitignored, auto-generated) |
| `instrument.necro` | Necrobrowser JSON template with placeholder tokens |
| `run_test.sh` | Test runner: checks prereqs, generates certs, starts muraena, runs tests |
| `integration_test.go` | Go integration test suite |
