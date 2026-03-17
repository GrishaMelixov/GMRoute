# GMRoute

A smart SOCKS5 proxy with domain-based routing, SNI sniffing, and auto-failover.

```
Client → GMRoute (SOCKS5) → Direct  ──→ Internet
                           ↘ Upstream → VPN/Proxy → Internet
```

## Features

- **SOCKS5 proxy** — accepts standard SOCKS5 connections on a configurable port
- **Domain routing** — route specific domains through an upstream proxy, everything else direct
- **Wildcard matching** — `youtube.com` rule also matches `www.youtube.com`, `m.youtube.com`, etc.
- **SNI sniffing** — extracts the real hostname from HTTPS traffic without decryption
- **Auto-failover** — if direct fails, automatically retries via upstream and caches the result
- **Live dashboard** — real-time traffic stats at `http://localhost:9090`

## Quick Start

### 1. Build

```bash
git clone https://github.com/GrishaMelixov/GMRoute.git
cd GMRoute
go build -o gmroute ./cmd/gmroute
```

Requires Go 1.21+.

### 2. Configure

Edit `config.yaml`:

```yaml
port: 1080        # SOCKS5 port to listen on
upstream: ""      # upstream SOCKS5 proxy, e.g. "127.0.0.1:7890"

rules:
  - domain: youtube.com
    route: upstream
  - domain: instagram.com
    route: upstream
```

Leave `upstream` empty to run in direct-only mode (useful for testing).

### 3. Run

```bash
./gmroute -config config.yaml
```

```
2025/01/01 12:00:00 proxy listening on :1080 | upstream="" | rules=2
2025/01/01 12:00:00 dashboard: http://localhost:9090
```

### 4. Configure your browser

Set SOCKS5 proxy to `127.0.0.1:1080` in your browser or system settings:

- **macOS**: System Settings → Network → Proxies → SOCKS Proxy → `127.0.0.1:1080`
- **Firefox**: Settings → Network Settings → Manual proxy → SOCKS Host `127.0.0.1` port `1080`
- **Chrome**: Use a proxy extension like SwitchyOmega

### 5. Open the dashboard

Visit [http://localhost:9090](http://localhost:9090) to see live traffic stats.

## Using with V2RayTun / V2Ray

If you have V2RayTun or another Xray/V2Ray client running locally, it typically exposes a local SOCKS5 port (commonly `127.0.0.1:7890` or `127.0.0.1:10808`). Set that as your upstream:

```yaml
upstream: "127.0.0.1:7890"

rules:
  - domain: youtube.com
    route: upstream
  - domain: twitter.com
    route: upstream
```

GMRoute then acts as a smart split-routing layer on top — blocked sites go through your VPN, everything else goes direct.

## Architecture

```
internal/
├── proxy/      # TCP server + SOCKS5 handshake handler
├── router/     # Routing engine: direct vs upstream decision
├── trie/       # Prefix tree for fast wildcard domain matching
├── sniffer/    # TLS ClientHello parser for SNI extraction
├── failover/   # Retry + per-domain route cache
├── metrics/    # Atomic counters for observability
├── config/     # YAML config loader
└── dashboard/  # HTTP dashboard with Server-Sent Events
```

## Running Tests

```bash
go test ./...
```
