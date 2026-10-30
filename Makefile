.PHONY: build release dashboard site frontend-install frontend-check frontend-test test fmt fmt-check lint go-lint dashboard-lint clean install

build: dashboard
	go build -o nebprx ./cmd/nebprx

# release mirrors .github/workflows/release.yml: strip the symbol
# table and DWARF (-s -w) and rewrite source paths (-trimpath). On
# this tree that drops the binary from ~88 MB to ~62 MB without
# changing runtime behaviour. Use this when packaging for users —
# `make build` stays unstripped so panics keep file:line info.
release: dashboard
	go build -ldflags "-s -w" -trimpath -o nebprx ./cmd/nebprx

dashboard:
	cd dashboard && pnpm install --frozen-lockfile && pnpm exec vp run build

site: dashboard
	cd site && pnpm install --frozen-lockfile && pnpm exec vp run build

frontend-install:
	cd dashboard && pnpm install --frozen-lockfile
	cd site && pnpm install --frozen-lockfile

frontend-check: frontend-install
	cd dashboard && pnpm exec vp check
	cd site && pnpm exec vp check

frontend-test: frontend-install
	cd dashboard && pnpm exec vp test
	cd site && pnpm exec vp test

test: dashboard frontend-test
	go test ./...

# Our tracked Go sources, excluding the vendored upstream copy under
# third_party/ (a verbatim tailscale.com tree that must not be
# reformatted — see third_party/tailscale/PATCH.md).
GO_SRC = $(shell git ls-files '*.go' | grep -v '^third_party/')

fmt:
	gofmt -w $(GO_SRC)
	cd dashboard && pnpm exec vp fmt
	cd site && pnpm exec vp fmt

fmt-check:
	test -z "$$(gofmt -l $(GO_SRC))"
	cd dashboard && pnpm exec vp fmt --check
	cd site && pnpm exec vp fmt --check

lint: go-lint frontend-check

# `go-lint` requires the dashboard bundle so the //go:embed in
# dashboard/embed.go resolves at compile time. Depending on
# `dashboard` here makes a clean checkout work without manual ordering.
go-lint: dashboard
	golangci-lint run --timeout 5m ./...

dashboard-lint:
	cd dashboard && pnpm exec vp check

clean:
	rm -f nebprx
	rm -rf dashboard/dist dashboard/node_modules site/dist site/node_modules

install: build
	install -m 0755 nebprx $${PREFIX:-$$HOME/.local/bin}/nebprx
