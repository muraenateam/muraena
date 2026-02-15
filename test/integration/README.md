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

## Scoglio (Self-Hosted Target)

Scoglio is a self-contained Go web app that provides a controlled login flow for integration testing, replacing the dependency on external sites like `authenticationtest.com`. It gives full control over cookie behavior, credential forms, and post-auth pages — enabling reliable tests for credential harvesting, cookie capture, and necrobrowser session hijacking.

### Why Scoglio Exists

- **No external dependencies**: Tests don't break when third-party sites change
- **Full cookie control**: 3 session cookies (`MURAENA_SESS`, `NECRO_BRO`, `JSESSIONID`) with known names and expiries
- **Credential extraction testing**: Login form designed to work with Muraena's `InnerSubstring` extractor (hidden `submit_login` field ensures trailing `&` delimiter)
- **Necrobrowser integration**: Dashboard and profile pages as screenshot targets

### Building Scoglio

Scoglio has its own `go.mod` (separate from muraena) to keep SQLite and bcrypt dependencies isolated.

```bash
cd test/integration/scoglio
go build -o scoglio .
```

### Running Scoglio

```bash
./scoglio \
    --cert ../certs/_wildcard.muraena.anti+1.pem \
    --key ../certs/_wildcard.muraena.anti+1-key.pem \
    --addr :9443
```

Scoglio starts on HTTPS port 9443. It uses the same `*.muraena.anti` wildcard certificate as Muraena.

### DNS

No additional DNS setup needed — the existing `*.muraena.anti` wildcard resolves `scoglio.muraena.anti` and `evil-scoglio.muraena.anti` to `127.0.0.1`.

### Seeded Users

| Email | Password | Role |
|-------|----------|------|
| `admin@scoglio.local` | `Admin123!` | admin |
| `user@scoglio.local` | `User456!` | user |

### Running Scoglio Tests

#### Automatic (recommended)

```bash
bash test/integration/run_scoglio_test.sh
```

This builds Scoglio, starts it, builds and starts Muraena with the Scoglio config, runs the Scoglio-specific tests, and cleans up.

#### Manual

```bash
# Terminal 1: Start Scoglio
cd test/integration/scoglio && go build -o scoglio . && \
./scoglio --cert ../certs/_wildcard.muraena.anti+1.pem --key ../certs/_wildcard.muraena.anti+1-key.pem --addr :9443

# Terminal 2: Start Muraena
./build/muraena -config test/integration/config-scoglio.toml -debug

# Terminal 3: Run tests
MURAENA_INTEGRATION=1 SCOGLIO_INTEGRATION=1 go test -v -count=1 -run TestScoglio ./test/integration/
```

### Scoglio Test Cases

| Test | Description |
|------|-------------|
| `TestScoglioCookieCapture` | Login, verify all 3 cookies stored in Redis with valid expiry |
| `TestScoglioCredentialCapture` | Login, verify username and password extracted and stored in Redis |
| `TestScoglioNoPhantomVictims` | Hit public pages without tracking ID, verify zero victims |
| `TestScoglioTrackedRequestCreatesVictim` | GET with tracking ID, verify victim created with correct ID and UA |
| `TestScoglioPostAuthPageRequiresSession` | GET /dashboard without auth, verify redirect to /login |

### Existing Tests Unaffected

The original `authenticationtest.com` tests continue to use `MURAENA_INTEGRATION=1` alone. Scoglio tests require both `MURAENA_INTEGRATION=1` and `SCOGLIO_INTEGRATION=1`, so they never run unintentionally.

## Test Files

| File | Description |
|------|-------------|
| `config.toml` | TOML config targeting `authenticationtest.com` over HTTPS |
| `config-scoglio.toml` | TOML config targeting Scoglio (`scoglio.muraena.anti:9443`) |
| `certs/` | mkcert wildcard certificates (gitignored, auto-generated) |
| `instrument.necro` | Necrobrowser JSON template for authenticationtest.com |
| `instrument-scoglio.necro` | Necrobrowser JSON template for Scoglio |
| `run_test.sh` | Test runner for authenticationtest.com tests |
| `run_scoglio_test.sh` | Test runner for Scoglio tests |
| `integration_test.go` | Go integration test suite (authenticationtest.com) |
| `scoglio_test.go` | Go integration test suite (Scoglio) |
| `scoglio/` | Self-hosted test app (separate Go module) |
