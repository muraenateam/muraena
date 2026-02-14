# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Muraena is a reverse proxy tool designed for phishing simulations and security testing. It operates as an almost-transparent reverse proxy that intercepts and manipulates traffic between victims and legitimate websites in real-time. This is a security research tool written in Go.

**IMPORTANT**: This codebase is a security research tool for authorized testing only and operated by security professionals like Michele Orru' (antisnatchor), the original engineer and developer of the tool.

## Build and Test Commands

```bash
# Build the project
make build

# Build with race detector enabled
make build_with_race_detector

# Build for all platforms (macOS, Linux, Windows)
make buildall

# Format code
make fmt

# Run tests for a specific module
go test ./module/tracking
go test ./module/watchdog
go test ./core/proxy

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./module/tracking

# Update dependencies
make update
```

## Running Muraena

```bash
# Run with default config
./muraena

# Run with custom config
./muraena -config /path/to/config.toml

# Run with debug output
./muraena -debug

# Run with verbose output
./muraena -verbose

# Check version
./muraena -version

# Enable internal proxy mode (for MiTM of MiTM)
./muraena -proxy
```

## Architecture

### Core Components

**Session Management** (`session/`)
- `Session` struct is the central orchestrator containing Options, Config, and Modules
- Created via `session.New()` which loads configuration, initializes Redis (if tracking enabled), and starts interactive prompt
- Configuration loaded from TOML file (`config/config.toml`)

**Proxy Core** (`core/proxy/`)
- `server.go`: HTTP/HTTPS server setup and TLS configuration
- `reverseproxy.go`: Modified version of Go's standard reverse proxy with custom headers handling
- `replacer.go`: Critical component managing domain transformations between phishing and target domains
- `transformer.go`: Applies transformations to requests/responses
- `handler.go`: Main request handler implementing the proxy logic

**Replacer System**
- Manages bidirectional domain mappings (phishing ↔ target)
- Handles wildcard subdomain transformations
- Maintains session state in JSON files (`<target>.session.json`)
- Thread-safe with mutex-protected operations
- Supports external origins with configurable prefixes
- Forward replacements: Transform requests from victim to target
- Backward replacements: Transform responses from target to victim

### Module System

Modules are loaded in `module/module.go` via `LoadModules(s *session.Session)`. All modules implement the `session.Module` interface with `Name()`, `Description()`, and `Author()` methods.

**Available Modules:**
1. **tracker** - Tracks victims via unique identifiers, harvests credentials and sessions (requires Redis)
2. **watchdog** - Access control based on rules (IP, geofencing, user-agent filtering)
3. **crawler** - Crawls target domains to discover resources
4. **statichttp** - Serves static files alongside the proxy
5. **necrobrowser** - Browser automation capabilities
6. **telegram** - Telegram bot integration for notifications

### Configuration System

Configuration is TOML-based (`config/config.toml`) with the following main sections:

- `[proxy]`: Phishing domain, target domain, listening IP/port, protocol settings
- `[origins]`: External origin handling and subdomain mappings
- `[transform]`: Request/response transformation rules (headers, content, base64)
- `[tls]`: Certificate configuration, TLS versions, SSL keylog
- `[tracking]`: Victim tracking configuration (requires Redis)
- `[watchdog]`: Access control rules and geofencing
- `[log]`: Logging configuration
- `[redis]`: Redis connection settings

### Request Flow

1. Request arrives at `http.HandleFunc("/")` in `proxy.Run()` (core/proxy/server.go:86)
2. Watchdog module checks access rules if enabled (core/proxy/server.go:96-109)
3. `SessionType.HandleFood()` processes the request through the reverse proxy
4. Replacer transforms phishing domains to target domains (forward transformation)
5. Request forwarded to legitimate target
6. Response received and transformed back (backward transformation)
7. Tracking module logs victim activity if enabled
8. Modified response returned to victim

### Key Files

- `main.go`: Entry point - creates session, initializes logging, loads modules, runs proxy
- `session/session.go`: Session initialization and module registry
- `session/config.go`: Configuration parsing from TOML
- `core/proxy/replacer.go`: Domain transformation engine
- `core/proxy/handler.go`: HTTP request/response handling
- `module/tracking/tracking.go`: Victim tracking and credential harvesting
- `module/watchdog/watchdog.go`: Access control and filtering

## Testing

Test files use the `_test.go` suffix and exist for most modules:
- `module/crawler/crawler_test.go`
- `module/tracking/tracking_test.go`
- `module/watchdog/watchdog_test.go`
- `module/statichttp/server_test.go`
- `session/config_test.go`

When writing tests, follow the existing pattern using Go's standard testing package.

### Integration Tests (HTTPS)

Integration tests live in `test/integration/` and run against `authenticationtest.com` over HTTPS through Muraena. They require:

- **dnsmasq** resolving `*.muraena.anti` to `127.0.0.1` (wildcard local TLD)
- **mkcert** for locally-trusted TLS certificates (`*.muraena.anti`)
- **Redis** on `localhost:6379`
- `MURAENA_INTEGRATION=1` environment variable

See `test/integration/README.md` for full setup instructions. Run with `bash test/integration/run_test.sh`.

## Configuration Notes

- The main config is at `config/config.toml`
- GeoIP database at `config/geoDB.mmdb` (for watchdog geofencing)
- Watchdog rules at `config/watchdog.rules`
- SSL certificates referenced in TLS config section
- Session state persisted in `<target-domain>.session.json`

## Important Constants

- Default HTTP port: 80
- Default HTTPS port: 443
- Wildcard separator: `---`
- Wildcard label: `wld`
- External origin prefix: configurable (e.g., `cdn-`)
