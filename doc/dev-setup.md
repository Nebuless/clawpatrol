# Local development setup

PRX is a single statically-linked Go binary. The dashboard
SPA is built separately with Vite and embedded into the binary at
build time.

## Prerequisites

- Go (see `go.mod` for the required version)
- Node.js 22.23.4 (pinned in `.node-version`) and pnpm 12.16.1.
  Vite+ 1.4.0 is installed locally from each application's pinned catalog;
  a global `vp` installation is optional. Deno is not required.
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
make test       # dashboard build + frontend suites + go test ./...
make frontend-install # install both pnpm locks without resolving new versions
make frontend-check   # Vite+ formatting, lint and type checks for both apps
make frontend-test    # Vitest suites for dashboard, site, and demo behavior
make site       # dashboard demo + website, docs and prerendered pages
make fmt        # gofmt + both frontend formatters
make fmt-check  # CI's format gate
make clean      # drop binary + both frontend dist/node_modules trees
make install    # PREFIX=~/.local/bin (override with PREFIX=...)
```

`make test` builds the dashboard and runs both frontend suites before Go tests.
The dashboard build runs `pnpm install --frozen-lockfile` and the local Vite+
build task in `dashboard/`. That task retains TypeScript checking before bundling.
A fresh checkout must build the dashboard before `go build ./cmd/nebprx`;
otherwise Go's embed cannot resolve `dashboard/dist`.

Each application retains its own `pnpm-lock.yaml` and `pnpm-workspace.yaml`.
The workspace files pin the Vite+ toolchain catalog, Vite alias and explicit
native dependency build approvals. Review both lockfiles when updating tools;
do not edit resolved versions or integrity hashes by hand.

Use `pnpm exec vp run dev` in either application for development. In the site,
this task generates example snippets before starting the dev server. Use
`pnpm exec vp run build` for the complete site pipeline: generation, TypeScript,
Vite, docs/prerendering and demo assembly. A bare `vp build` runs only the bundler.
Runtime Go binaries do not need Node.js, pnpm or Vite+ installed.

For this project's shared development environment, run heavy builds, tests and
browser checks on EGGIE through `ssh root@eggie`. See `.node-version` and the
application manifests for portable tool pins; machine-local tool directories
are not installation requirements.

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
test (`go test ./internal/tools/docgen/...`) fails if you change a
`Plugin.New()` body struct without regenerating
`site/doc/config-reference.md` — run `go run ./internal/tools/docgen` to fix.

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
