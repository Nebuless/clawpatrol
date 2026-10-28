# Local development setup

PRX is a single statically-linked Go binary. The dashboard
SPA is built separately with Vite and embedded into the binary at
build time.

## Prerequisites

- Go (see `go.mod` for the required version)
- [Deno](https://deno.com) v2+ (only required if you want to rebuild
  the dashboard SPA in `dashboard/`). Deno reads `dashboard/package.json`
  directly; npm isn't needed.
- Docker with Compose (optional, for end-to-end testing against an
  in-container agent)

On macOS, `nebprx gateway` runs the service. `nebprx join --whole-machine`
routes host traffic through a WireGuard gateway using `wg-quick` (requires
`wireguard-tools` and `wireguard-go`). Plain `join` saves the configuration
without routing. Per-command `nebprx run` is Linux-only. Tailscale client
routing is not supported on macOS.

## Build

```sh
make            # builds dashboard SPA + Go binary -> ./nebprx
make release    # release-shaped build: dashboard + go build -ldflags '-s -w' -trimpath
make test       # go test ./...
make fmt        # gofmt + dashboard formatter
make fmt-check  # CI's format gate
make clean      # drop ./nebprx, dashboard/dist, dashboard/node_modules
make install    # PREFIX=~/.local/bin (override with PREFIX=...)
```

Under the hood, the dashboard build runs `deno install && deno task
build` in `dashboard/`. Skip it (`go build ./cmd/nebprx` directly)
and the Go embed ships a placeholder.

`make build` keeps debug symbols and DWARF for panics with file:line
info. `make release` matches what `.github/workflows/release.yml`
ships and what `install.sh` builds from source: stripped (`-s -w`)
and trimpath'd, which roughly drops the binary from ~88 MB to ~62 MB
on Linux/amd64. See [doc/binary-size.md](binary-size.md) for the
size baseline and the list of intentionally heavy dependencies.

## Quick start

Copy the example config into a local data directory, edit the
operational fields (listen ports, `public_url`, `wg_endpoint`,
`state_dir`), then run:

```sh
mkdir -p ./data
cp examples/gateway.example.hcl ./data/gateway.hcl
$EDITOR ./data/gateway.hcl
./nebprx gateway ./data/gateway.hcl
```

Ports used by the example:

| What | Port |
|---|---|
| Dashboard + HTTP API | `tcp/9080` |
| TLS MITM listener | `tcp/8443` |
| WireGuard listener | `udp/51820` |

Dashboard: <http://localhost:9080>.

Tests live alongside the code: `go test ./...`. The docgen drift
test (`go test ./tools/docgen/...`) fails if you change a
`Plugin.New()` body struct without regenerating
`site/doc/config-reference.md` — run `go run ./tools/docgen` to fix.

## Testing with a Docker agent

Build and run openclaw against your local gateway:

```sh
cd /path/to/openclaw
docker build -t openclaw:local .
mkdir -p /tmp/openclaw-dev/{config,workspace}
echo '{"gateway":{"mode":"local"}}' > /tmp/openclaw-dev/config/openclaw.json
OPENCLAW_CONFIG_DIR=/tmp/openclaw-dev/config \
OPENCLAW_WORKSPACE_DIR=/tmp/openclaw-dev/workspace \
docker compose up -d openclaw-gateway
```

Join the openclaw container against your local gateway:

```sh
docker exec <openclaw-container> nebprx join \
  http://host.docker.internal:9080
```

Verify interception:

```sh
docker exec <openclaw-container> curl -sf https://httpbin.org/get
# then check http://localhost:9080/requests for the captured action
```
