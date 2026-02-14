# Integration Tests

## Prerequisites

- **Redis** running on `localhost:6379` (default)
- `/etc/hosts` entry: `127.0.0.1 evil-authtest.local`
- Environment variable: `MURAENA_INTEGRATION=1` (tests are skipped without it)
- Target: `authenticationtest.com` (public test site)

## Running Tests

### Automatic (recommended)

```bash
./test/integration/run_test.sh
```

### Manual

```bash
export MURAENA_INTEGRATION=1
go test -v ./test/integration/
```

## TLS Configuration

Tests run with TLS disabled (`[tls] enable = false`) to avoid self-signed certificate complexity. The test HTTP client uses `InsecureSkipVerify=true`, so both TLS and plaintext modes work.

To test with TLS enabled:

```bash
# Generate self-signed certs
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj '/CN=evil-authtest.local'

# Update config.toml: set [tls] enable = true and provide cert/key/root inline
```

## Necrobrowser Integration Testing

To test the necrobrowser integration (post-phishing automation):

1. Install necrobrowser: `cd /path/to/necrobrowser && npm install`
2. Start necrobrowser: `node necrobrowser.js`
3. In `config.toml`, set `[necrobrowser] enable = true` and configure `endpoint` to point to necrobrowser's `/instrument` URL
4. The `instrument.necro` template defines the JSON payload sent to necrobrowser, with placeholders (`%%%TRACKER%%%`, `%%%COOKIES%%%`, `%%%USERAGENT%%%`) replaced at runtime

**Same-IP requirement**: In production, Muraena and Necrobrowser MUST share the same exit IP (same machine, or VPN tunnel). Target sites check for IP consistency across sessions; IP mismatch between the victim's session and necrobrowser's replay is a common anti-hijacking detection.

## Test Files

| File | Description |
|------|-------------|
| `config.toml` | TOML configuration targeting `authenticationtest.com` |
| `instrument.necro` | Necrobrowser JSON template with placeholder tokens |
| `run_test.sh` | Shell script to start Redis, run tests, clean up |
| `integration_test.go` | Go integration test suite |
