# GMRoute

> Intelligent SOCKS5 proxy router written in Go — routes traffic based on domain rules, sniffs TLS without decryption, and visualizes connections on a real-time 3D globe.

---

## What is it?

GMRoute is a local SOCKS5 proxy that sits between your browser and the internet. Instead of blindly forwarding all traffic through one path, it **decides per-domain** where to send each connection:

```
Browser → GMRoute :1080 → (routing decision) → Direct
                                              → Upstream proxy (VPN/SOCKS5)
```

Point your browser at `localhost:1080`, open `localhost:9090`, and watch live traffic arcs fly across the globe.

---

## Features

- **SOCKS5 core** — full protocol implementation (IPv4, IPv6, domain), no external dependencies
- **Trie-based domain routing** — O(k) wildcard matching, `*.google.com` just works
- **SNI sniffing** — extracts the real hostname from TLS ClientHello *without decrypting traffic*
- **Auto-failover** — if direct fails, retries via upstream and caches the result per-domain
- **YAML config** — edit rules in a file, no recompile needed
- **Real-time dashboard** — 3D globe with live traffic arcs, connection log, route filters
- **Settings panel** — add/remove routing rules from the browser, auto-saved to `config.yaml`
- **Graceful shutdown** — drains active connections before exit
- **Zero allocations on hot path** — `sync.Pool` for buffers, `io.CopyBuffer` for zero-copy tunneling

---

## Quick Start

**Prerequisites:** Go 1.21+

```bash
git clone https://github.com/GrishaMelixov/GMRoute.git
cd GMRoute
go run ./cmd/gmroute
```

Then:
1. Set your browser SOCKS5 proxy to `127.0.0.1:1080`
2. Open `http://localhost:9090` to see the dashboard
3. Browse — watch connections appear on the globe in real time

**macOS:** System Settings → Network → Wi-Fi → Details → Proxies → SOCKS Proxy → `127.0.0.1:1080`

---

## Configuration

Edit `config.yaml`:

```yaml
# Port to listen on
port: 1080

# Upstream SOCKS5 proxy (e.g. a local V2Ray/Xray client)
upstream: "127.0.0.1:7890"

# Per-domain routing rules
# Subdomains match automatically: youtube.com also matches www.youtube.com
rules:
  - domain: youtube.com
    route: upstream
  - domain: instagram.com
    route: upstream
  - domain: x.com
    route: upstream
```

Rules can also be managed live from the **Settings** panel in the dashboard without restarting.

---

## Dashboard

Open `http://localhost:9090` while the proxy is running.

| Feature | Description |
|---|---|
| **3D Globe** | Live arcs showing traffic destinations. Green = direct, blue = upstream |
| **Country labels** | Semi-transparent country names on the globe |
| **Connection log** | Domain, country, route type, timestamp |
| **Filters** | Switch between All / Direct / Upstream |
| **Settings** | Add/remove routing rules live — saved to `config.yaml` automatically |

---

## How It Works

### Routing Engine

Every connection goes through a **Trie** (prefix tree) that matches domains in reverse-label order (`com → youtube → www`). O(k) lookup where k is the number of domain labels — constant regardless of how many rules exist.

### SNI Sniffing

When a client connects to a raw IP over TLS, GMRoute reads the first bytes of the TLS handshake (ClientHello), extracts the **Server Name Indication** field, and uses it for routing — without decrypting any traffic. A `PeekConn` wrapper replays the bytes back into the stream transparently.

### Auto-Failover

```
Resolve(domain) → direct → dial fails → retry via upstream → cache result
```

Successful fallbacks are stored in a `sync.Map` per-domain, so future connections skip the failed path immediately.

### Real-time Dashboard

Each successful connection triggers an async goroutine:
1. DNS resolve domain → IP
2. Geo lookup IP via ip-api.com (in-memory cache)
3. Emit typed Server-Sent Event to all connected dashboard clients

The frontend (globe.gl + vanilla JS) draws animated arcs between your location and the destination.

---

## Project Structure

```
GMRoute/
├── cmd/gmroute/         # Entry point
├── internal/
│   ├── proxy/           # SOCKS5 server + connection handler
│   ├── router/          # Routing engine (domain → route decision)
│   ├── trie/            # Generic prefix-tree for domain matching
│   ├── sniffer/         # TLS ClientHello SNI extractor
│   ├── failover/        # Retry + per-domain route cache
│   ├── connlog/         # Connection event bus (ring buffer + pub/sub)
│   ├── geo/             # IP geolocation with in-memory cache
│   ├── metrics/         # Atomic counters (active/total/direct/upstream/errors)
│   ├── config/          # YAML config loader
│   └── dashboard/       # HTTP server + SSE + 3D globe UI
└── config.yaml          # Default configuration
```

---

## Tech Highlights

- **No frameworks** — standard library only (`net`, `net/http`, `sync`, `context`)
- **Generics** — `Trie[T any]` works for any value type
- **`sync/atomic`** — lock-free metrics counters
- **Server-Sent Events** — lightweight real-time push without WebSocket overhead
- **`io.CopyBuffer` + `sync.Pool`** — reusable 32KB buffers, minimal GC pressure
- **Context-based shutdown** — `signal.NotifyContext` propagates OS signals to the listener

---

## Running Tests

```bash
go test ./...
```

---

## License

MIT
