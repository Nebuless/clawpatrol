# Project conventions

This file is the single source of truth for repo-level agent
instructions. `AGENTS.md` at the repo root is a symlink to this file
so Codex and Claude Code both pick it up.

## Dates and times

Render every timestamp in the UI as `yyyy-MM-dd HH:mm:ss.SSS` (24-hour,
locale-independent, millisecond precision). When only the time-of-day
is shown, use `HH:mm:ss.SSS`.

Helpers live in `cmd/nebprx/www/src/lib/format.ts`:

- `fmtDateTime(t)` — full timestamp.
- `fmtTime(t)` — time-of-day only.

Do not call `toLocaleDateString`, `toLocaleTimeString`, or
`toLocaleString` for date/time rendering. They produce different
output per locale (`5/11/2026` in en-US, `11/05/2026` in en-GB,
`11.5.2026` in de-DE), which makes log entries unreadable across a
team. Number formatting (`Intl.NumberFormat`, `Number.toLocaleString`)
is fine — the rule is about dates and times.

## Formatting gates

CI fails fast on unformatted code. Before pushing, run the same
checks the lint job runs:

- Go: `gofmt -l .` from the repo root. Empty output means clean;
  any filename listed is a fail. Fix with `gofmt -w .`.
- Dashboard (TS/JS/HTML/JSON in `dashboard/`):
  `cd dashboard && deno task format:check`. Fix with `deno task format`.

Either fail will block the `test` workflow before the actual tests
run. Cheap to check locally; expensive in CI round-trips.

## `cmd/nebprx/testdata/`

The self-test corpus for `nebprx test` (see `doc/test.md`).
`example.hcl` is the policy under test; the `*.json` files are
action fixtures. The intent is one fixture per CEL branch in
`example.hcl` — every rule's `allow` and `deny` arm exercised at
least once. Adding a rule means adding fixtures.

Generating new fixtures: run the gateway against `example.hcl`,
issue the request you want to capture, click "Download action"
on the dashboard's request detail page, drop the file into
`testdata/`. Don't hand-edit recorded JSON; if the format
changes, regenerate.

Scope: keep it minimal and synthetic — public hostnames, no real
IPs, no production credentials.

## Brand assets

Brand sources live in `shared/branding/` (see its README), with colors
in `shared/tokens.css`, treatments in `shared/brand.css`, and self-hosted
fonts in `shared/fonts/`. `site/public/` and `dashboard/public/` reference
the shared assets through symlinks.

Use `prx-icon.svg` on light surfaces, `prx-icon-light.svg` on dark
surfaces, and `prx-icon-mono.svg` for one-color/template uses. Use
`prx-lockup.svg` / `prx-lockup-light.svg` for outlined lockups,
`prx-app-icon.svg` and its PNG exports for app tiles, the favicon and
apple-touch exports for browser icons, and `og-image.png` for link previews.

Product prose says "PRX" or "PRX by Nebuless". The CLI is `nebprx`,
environment variables use `NEBPRX_*`, and the install URL is
`https://prx.nebuless.cloud/install.sh`. The Go module is
`github.com/Nebuless/nebprx`. Legacy `claw-patrol-*` assets are
kept only for compatibility.
