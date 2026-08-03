# muraena-recon (Puppeteer recon crawler)

An **optional host-side dependency** used by the Muraena API to auto-discover a
target's reverse-proxy origins and login-page intercept patterns. Puppeteer is
not required to build or run the Go API — it is only needed when you actually
trigger a recon job (`POST /api/v1/recon`). A host without Node/puppeteer simply
gets a clear `503 node not found` from the recon endpoint.

## Install

Install inside this directory (pulls Chromium via puppeteer):

```bash
cd puppeteer
npm install
```

## Standalone usage

```bash
node recon.js --target https://example.com --depth 1 --max-pages 20 --timeout 20000
```

- Prints **one JSON document** to stdout on completion with keys:
  `target`, `origins` (external hosts, excluding the target host), `loginPages`,
  `secretsPaths`, `secretsPatterns`.
- Writes progress lines to stderr as `{"type":"progress","msg":...}` for streaming.

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `--target <url>` | (required) | `http(s)` URL to crawl |
| `--depth <n>` | `1` | BFS crawl depth for same-host links |
| `--max-pages <n>` | `20` | Hard cap on pages visited |
| `--timeout <ms>` | `20000` | Per-page navigation timeout |

## How the Go API invokes it

The API spawns this script as a subprocess (argv array, **no shell**), using the
`[recon]` config section:

```toml
[recon]
nodePath = "node"            # path to the Node.js binary (resolved via PATH)
script   = "puppeteer/recon.js"
```

The runner executes:

```
<nodePath> <script> --target <target> --depth <depth> --max-pages <maxPages>
```

then streams the stderr progress lines over `WS /api/v1/ws/recon`, parses the
stdout JSON, and merges the discovered origins + secrets patterns into the live
config via the Phase 2 config pipeline.
