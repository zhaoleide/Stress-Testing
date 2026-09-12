# stress-testing

Config-driven HTTP load testing for Go. Describe a request in YAML or JSON, then run it from the CLI with either a fixed request count or a time-boxed duration. SSH system monitoring is optional and off by default.

## Features

- Generic HTTP scenario: method, URL/path, headers, body, timeout, TLS skip-verify
- Load modes: `requests` (N calls) and `duration` (run until the clock expires)
- Concurrency and optional target RPS
- Live progress plus success rate, QPS, min/avg/max, **P50/P90/P95/P99**
- HTTP status and error-class counts
- Reports: HTML, JSON, CSV
- Optional SSH monitor (CPU/memory/disk/network) driven only by config

## Install / build

```bash
go build -o stress-testing ./cmd/stress-testing
```

Requires Go 1.23+.

## CLI

```bash
./stress-testing run -c examples/http-get.yaml
./stress-testing run -c examples/http-post.json --out /tmp/reports
./stress-testing validate -c examples/login.yaml
./stress-testing version
```

Flags (override the config file):

| Flag | Meaning |
| --- | --- |
| `-c` / `-config` | YAML or JSON config path (required for `run` and `validate`) |
| `-concurrency` | Worker count |
| `-requests` | Request count for `mode: requests` |
| `-duration` | Duration such as `30s` |
| `-out` | Report directory (default `reports/`) |

`go run .` still works and forwards to the same CLI; `./cmd/stress-testing` is the supported entrypoint.

## Example configs

See `examples/`:

- `http-get.yaml` — GET, fixed request count
- `http-post.json` — POST, duration mode
- `login.yaml` — login-style POST with `${TOKEN}` / `${LOGIN_PASSWORD}`

Do not put real passwords, keys, or private IPs in the repo. Expand secrets from the environment:

```yaml
headers:
  Authorization: "Bearer ${TOKEN}"
monitor:
  password: "${SSH_PASSWORD}"
```

## Config schema

```yaml
name: api-smoke
target:
  base_url: https://example.com   # or set target.url
  path: /api/v1/ping
  method: GET
  headers:
    Content-Type: application/json
  body: ""                        # optional JSON string
  body_file: ""                   # optional path to a request body file
  timeout: 10s
  insecure_skip_verify: false

load:
  mode: requests                  # requests | duration
  requests: 1000                  # required when mode=requests
  duration: 30s                   # required when mode=duration
  concurrency: 20
  # rate: 100                     # optional target RPS

monitor:                          # optional; default off
  enabled: false
  host: ""
  username: ""
  password: "${SSH_PASSWORD}"
  private_key: ""                 # path to a local key; do not commit keys
  interval: 2s
  metrics: [cpu, memory, disk, network]

report:
  formats: [html, json, csv]
  dir: reports
```

Required fields: `name`, `target.base_url` or `target.url`, `load.mode`, `load.concurrency`, plus `load.requests` or `load.duration` depending on mode. Enabling monitor also requires `host`, `username`, and either `password` or `private_key`.

## Reports

After a run, look in the report directory (default `reports/`):

- `report_*.html` — charts for latency, success, percentiles; system charts only if monitoring collected data
- `report_*.json` — full results and summary
- `stats_*.csv` — one row per stage, including P50/P90/P95/P99

## Custom scenarios

The engine still uses the `Scenario` / `Stage` interfaces in `framework/`. The default CLI path is `scenarios.HTTPScenario`. To add a custom flow, implement those interfaces and construct `framework.NewEngine(yourScenario)`.

## Safety

- This tool generates load. Start small and avoid production unless you intend to.
- SSH host-key verification is currently insecure-ignore (same as before); treat monitor credentials as sensitive.
- Public examples use `example.com` / `httpbin.org` placeholders only.

## License

MIT License
