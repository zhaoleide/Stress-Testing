# Stress-Testing Plan A Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Prefer frequent commits. Follow the design in `docs/superpowers/specs/2026-09-12-stress-testing-refactor-design.md`.

**Goal:** Evolve the existing Go framework into a general HTTP load tool driven by YAML/JSON + CLI, with requests and duration load modes, optional SSH monitoring, and richer latency percentiles.

**Architecture:** Keep `framework` (Engine / Monitor / Reporter / types). Add CLI (`cmd/stress-testing`), config loader (`internal/config`), shared HTTP client helpers (`internal/httpx`), and a config-driven `scenarios` HTTP scenario. Replace hardcoded `main.go` config; migrate login to `examples/`.

**Tech Stack:** Go 1.23, stdlib + existing `golang.org/x/crypto` for SSH; YAML via `gopkg.in/yaml.v3` (or equivalent); cobra/flag for CLI (stdlib `flag` is fine if kept small).

## Global Constraints

- No real passwords/keys in repo; use `${ENV}` expansion for secrets.
- SSH monitor default off.
- Support load modes: `requests` and `duration`; optional RPS limit.
- Reports: HTML + JSON + CSV; stats include P50/P90/P95/P99.
- `go test ./...` and `go build ./...` must pass.
- Public repo: never reintroduce internal IPs or credentials.
- Do not delete SSH monitor capability; make it config-driven.

## File map

- Create: `cmd/stress-testing/main.go`
- Create: `internal/config/config.go`, `internal/config/load.go`, `internal/config/load_test.go`
- Create: `internal/httpx/client.go`, `internal/httpx/request.go`
- Create: `scenarios/http_scenario.go`
- Create: `examples/http-get.yaml`, `examples/http-post.json`, `examples/login.yaml`
- Modify: `framework/types.go`, `framework/engine.go`, `framework/reporter.go` (percentiles, duration mode, progress)
- Modify: `framework/monitor.go` only as needed for config wiring
- Modify/replace: `main.go` → thin wrapper or remove in favor of `cmd/`; remove or gut `quick_main.go`
- Modify: `README.md`

---

### Task 1: Config model + loader

**Files:**
- Create: `internal/config/*`
- Test: `internal/config/load_test.go`

**Produces:** `Load(path) (*FileConfig, error)`, env expansion `${VAR}`, validate mode/fields.

- [ ] Write tests for YAML/JSON load, env expansion, validation failures
- [ ] Implement loader + validation
- [ ] `go test ./internal/config/...`
- [ ] Commit

### Task 2: HTTP helpers + HTTP scenario

**Files:**
- Create: `internal/httpx/*`, `scenarios/http_scenario.go`

**Produces:** Scenario implementing existing `framework.Scenario` / `Stage` from FileConfig.

- [ ] Implement HTTP client (timeout, insecure skip verify)
- [ ] Implement generic HTTP stage (method/url/headers/body)
- [ ] Wire Scenario Config()/Validate()/Stages()
- [ ] Tests for URL join / header merge where useful
- [ ] Commit

### Task 3: Engine load modes + percentiles

**Files:**
- Modify: `framework/types.go`, `framework/engine.go`

**Produces:** requests + duration execution; StageStats with P50/P90/P95/P99; optional rate limit; progress logs.

- [ ] Extend Config/Stats types
- [ ] Implement duration mode + optional RPS
- [ ] Percentile calculation from latencies
- [ ] Unit tests for stats/percentiles
- [ ] Commit

### Task 4: CLI entrypoints

**Files:**
- Create: `cmd/stress-testing/main.go`
- Modify: root `main.go` / remove `quick_main.go`

**Produces:** `run`, `validate`, `version` commands.

- [ ] Implement CLI
- [ ] `go build -o stress-testing ./cmd/stress-testing`
- [ ] Commit

### Task 5: Reporter + monitor wiring

**Files:**
- Modify: `framework/reporter.go`, monitor usage from config

**Produces:** Reports include new percentiles; monitor only when enabled.

- [ ] Update HTML/JSON/CSV with percentiles
- [ ] Ensure monitor stays optional
- [ ] Commit

### Task 6: Examples + README

**Files:**
- Create: `examples/*`
- Modify: `README.md`

- [ ] Add example configs (placeholders only)
- [ ] Document CLI usage and config schema
- [ ] Commit

### Task 7: End-to-end verification

- [ ] `go test ./...`
- [ ] `go build ./...`
- [ ] `stress-testing validate -c examples/http-get.yaml`
- [ ] Optional: short dry-run against `https://httpbin.org/get` if network allows
- [ ] Open PR summarizing changes

## Done when

- CLI runs from config for both load modes
- No hardcoded secrets
- Percentiles in console + reports
- SSH monitor optional
- PR opened against default branch
