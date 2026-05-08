#!/usr/bin/env python3
"""
inject-session.py — manually inject browser-exported cookies into Necrobrowser-NG.

Usage:
  ./inject-session.py                        # interactive: paste JSON cookies
  ./inject-session.py cookies.json           # from file
  ./inject-session.py cookies.json -u USER -p PASS
  ./inject-session.py cookies.json --tracker my-campaign-01
  ./inject-session.py --endpoint http://10.0.0.2:3000/instrument cookies.json

Cookie JSON format (browser extension export — Cookie-Editor, EditThisCookie, etc.):
  [
    {
      "name": "sessionToken",
      "value": "abc123",
      "domain": ".example.com",
      "path": "/",
      "secure": true,
      "httpOnly": true,
      "sameSite": "Lax",
      "expirationDate": 1700000000
    }
  ]

Netscape/curl cookie jar format is also supported (the tab-separated .txt files).
"""

import argparse
import json
import re
import sys
import time
import tomllib
import urllib.error
import urllib.request
from pathlib import Path

CONFIG_PATH = Path(__file__).parent / "config" / "config.toml"
PROFILE_PATH = Path(__file__).parent / "config" / "instrument.necro"

BOLD  = "\033[1m"
GREEN = "\033[32m"
CYAN  = "\033[36m"
RED   = "\033[31m"
RESET = "\033[0m"

def info(msg):  print(f"{GREEN}[+]{RESET} {msg}")
def error(msg): print(f"{RED}[✗]{RESET} {msg}", file=sys.stderr); sys.exit(1)
def header(msg): print(f"\n{BOLD}{CYAN}── {msg} ──{RESET}")


# ── Config loading ─────────────────────────────────────────────────────────────

def load_config():
    if not CONFIG_PATH.exists():
        return {}
    with open(CONFIG_PATH, "rb") as f:
        return tomllib.load(f)


def get_necro_settings(cfg, args):
    necro = cfg.get("necrobrowser", {})
    endpoint = args.endpoint or necro.get("endpoint")
    profile  = args.profile  or necro.get("profile")

    if not endpoint:
        error(
            "Necrobrowser endpoint not found.\n"
            "  Set it in config/config.toml under [necrobrowser] endpoint = \"...\"\n"
            "  or pass --endpoint http://host:3000/instrument"
        )
    if profile:
        profile_path = Path(profile.replace("./", "")).resolve()
    else:
        profile_path = PROFILE_PATH

    if not profile_path.exists():
        error(f"Profile file not found: {profile_path}\n"
              f"  Copy config/instrument.necro.example → config/instrument.necro and edit it.")

    return endpoint, profile_path


# ── Cookie parsing ─────────────────────────────────────────────────────────────

def parse_netscape_cookies(text):
    """Parse Netscape / curl cookie jar format."""
    cookies = []
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("\t")
        if len(parts) < 7:
            continue
        domain, _, path, secure, expires, name, value = parts[:7]
        cookies.append({
            "domain":         domain,
            "path":           path,
            "secure":         secure.upper() == "TRUE",
            "expirationDate": int(expires),
            "name":           name,
            "value":          value,
            "httpOnly":       False,
            "session":        int(expires) < 1,
        })
    return cookies


def parse_cookies(raw):
    """Accept JSON array or Netscape cookie jar format."""
    raw = raw.strip()
    if raw.startswith("[") or raw.startswith("{"):
        data = json.loads(raw)
        if isinstance(data, dict):
            data = [data]
        # Normalise: ensure expirationDate is an int, add session flag if missing
        out = []
        for c in data:
            c.setdefault("session", not bool(c.get("expirationDate")))
            c.setdefault("httpOnly", False)
            c.setdefault("secure", False)
            c.setdefault("path", "/")
            out.append(c)
        return out
    else:
        return parse_netscape_cookies(raw)


def read_cookies_interactive():
    header("Paste Cookies")
    print("  Paste your cookie JSON (from Cookie-Editor, EditThisCookie, etc.)")
    print("  or a Netscape cookie jar file. Press Enter twice when done.\n")
    lines = []
    blank = 0
    try:
        while blank < 2:
            line = input()
            if line == "":
                blank += 1
            else:
                blank = 0
            lines.append(line)
    except EOFError:
        pass
    return "\n".join(lines)


# ── Build and send request ─────────────────────────────────────────────────────

def build_body(profile_path, tracker, cookies, credentials, tokens):
    template = profile_path.read_text()
    cookies_json = json.dumps(cookies, indent=2)
    creds_json   = json.dumps(credentials)
    tokens_json  = json.dumps(tokens, indent=2)

    body = template
    body = body.replace("%%%TRACKER%%%",     tracker)
    body = body.replace("%%%COOKIES%%%",     cookies_json)
    body = body.replace("%%%CREDENTIALS%%%", creds_json)
    body = body.replace("%%%TOKENS%%%",      tokens_json)
    return body


def send_to_necrobrowser(endpoint, body):
    data = body.encode("utf-8")
    req  = urllib.request.Request(
        endpoint,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return resp.status, resp.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
    except urllib.error.URLError as e:
        error(f"Could not reach Necrobrowser-NG at {endpoint}:\n  {e.reason}")


# ── Main ───────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Manually inject browser cookies into Necrobrowser-NG",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("cookies_file", nargs="?",
                        help="Path to cookie JSON file (omit to paste interactively)")
    parser.add_argument("--tracker", "-t", default=None,
                        help="Tracker/session ID label (default: auto-generated)")
    parser.add_argument("--username", "-u", default="",
                        help="Captured username (optional)")
    parser.add_argument("--password", "-p", default="",
                        help="Captured password (optional)")
    parser.add_argument("--endpoint", "-e", default=None,
                        help="Override Necrobrowser-NG endpoint URL")
    parser.add_argument("--profile", default=None,
                        help="Override path to instrument.necro profile")
    parser.add_argument("--token", "-T", action="append", metavar="TYPE=VALUE",
                        help="OAuth/bearer token to include (repeatable). "
                             "Format: TYPE=VALUE  e.g. --token access_token=eyJ... "
                             "--token refresh_token=eyJ...")
    parser.add_argument("--dry-run", action="store_true",
                        help="Print the request body without sending it")
    args = parser.parse_args()

    print(f"\n{BOLD}{CYAN}Muraena — Manual Cookie Injection{RESET}\n")

    cfg = load_config()
    endpoint, profile_path = get_necro_settings(cfg, args)

    # ── Load cookies ────────────────────────────────────────────────────────
    if args.cookies_file and args.cookies_file != "-":
        raw = Path(args.cookies_file).read_text()
        info(f"Reading cookies from {args.cookies_file}")
    elif args.cookies_file == "-" or not sys.stdin.isatty():
        raw = sys.stdin.read()
        info("Reading cookies from stdin")
    else:
        raw = read_cookies_interactive()

    try:
        cookies = parse_cookies(raw)
    except json.JSONDecodeError as e:
        error(f"Invalid JSON: {e}")

    if not cookies:
        error("No cookies parsed — check the input format.")

    info(f"Parsed {len(cookies)} cookie(s)")

    # ── Credentials ─────────────────────────────────────────────────────────
    credentials = {"username": args.username, "password": args.password}

    # ── Tokens ───────────────────────────────────────────────────────────────
    tokens = []
    for t in (args.token or []):
        if "=" not in t:
            error(f"Invalid --token format '{t}'. Use TYPE=VALUE  e.g. access_token=eyJ...")
        token_type, _, token_value = t.partition("=")
        tokens.append({"type": token_type.strip(), "value": token_value.strip(),
                        "time": time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime())})
    if tokens:
        info(f"Tokens     : {', '.join(t['type'] for t in tokens)}")

    # ── Tracker ID ──────────────────────────────────────────────────────────
    tracker = args.tracker or f"manual-{int(time.time())}"
    info(f"Tracker ID : {tracker}")

    # ── Build body ──────────────────────────────────────────────────────────
    body = build_body(profile_path, tracker, cookies, credentials, tokens)

    if args.dry_run:
        header("Dry Run — Request Body")
        print(body)
        print()
        info(f"Would POST to: {endpoint}")
        return

    # ── Send ─────────────────────────────────────────────────────────────────
    info(f"Sending to Necrobrowser-NG → {endpoint}")
    status, response = send_to_necrobrowser(endpoint, body)

    print()
    if status in (200, 201):
        info(f"Success ({status})")
        task_id = response.strip()
        print(f"\n  {BOLD}Task ID  :{RESET} {task_id}")
        print(f"  {BOLD}Poll URL :{RESET} {endpoint.replace('/instrument', '')}/instrument/{task_id}")
        print(f"\n  Poll for completion:")
        base = endpoint.replace("/instrument", "")
        print(f"    curl {base}/instrument/{task_id}")
    else:
        print(f"{RED}[✗]{RESET} Necrobrowser-NG returned HTTP {status}:")
        print(f"    {response}")
        sys.exit(1)


if __name__ == "__main__":
    main()
