# telemt-exporter

Prometheus exporter for [telemt](https://github.com/telemt/telemt) — MTProxy on Rust.

Scrapes two telemt API endpoints and exposes Prometheus metrics on `:9101`.

## Metrics

### Per-token (from /v1/stats/users)

| Metric | Type | Labels | Description |
|---|---|---|---|
| `telemt_user_traffic_bytes_total` | Counter | `username` | Total traffic (upload + download) |
| `telemt_user_active_connections` | Gauge | `username` | Current active connections |

### Per-instance (from /v1/stats/zero/all)

| Metric | Type | Labels | Description |
|---|---|---|---|
| `telemt_connections_total` | Counter | — | Total connection attempts |
| `telemt_connections_bad_total` | Counter | — | Failed connection attempts |
| `telemt_connections_bad_by_class` | Counter | `class` | Failed connections grouped by reason |
| `telemt_handshake_failures_by_class` | Counter | `class` | Handshake failures grouped by reason |

### Health

| Metric | Type | Description |
|---|---|---|
| `telemt_scrape_up` | Gauge | 1 = both endpoints responded successfully |

## Usage

### Build and run locally

```bash
go mod tidy
go build -o telemt-exporter .
./telemt-exporter -url http://localhost:54321 -token YOUR_TOKEN
```

### Docker Compose

```bash
TELEMT_TOKEN=your_token docker compose up -d
```

Metrics are available at `http://localhost:9101/metrics`.

## Configuration

| Flag | Default | Description |
|---|---|---|
| `-url` | `http://localhost:54321` | telemt API base URL |
| `-token` | `""` | Bearer token for API auth |
| `-listen` | `:9101` | Address to expose metrics on |

## Grafana

Ready-to-import dashboard: [`grafana-dashboard.json`](grafana-dashboard.json)
(Grafana → Dashboards → New → Import).

Panels:
- Connection rate (connections/s)
- Good vs bad connections (green/red)
- Failure rate (% — thresholds at 5% and 10%)
- Active connections per token
- Traffic by token (bytes/s)
- Bad connections by reason (range total, top-10)
- Handshake failures by reason (range total, top-10)
- Scrape status (1 = OK)
