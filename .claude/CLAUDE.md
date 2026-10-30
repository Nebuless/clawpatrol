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
  `cd dashboard && pnpm exec vp fmt --check`. Fix with `pnpm exec vp fmt`.
  Run the same commands in `site/`. Install with `make frontend-install` first.

Either fail will block the `test` workflow before the actual tests
run. Cheap to check locally; expensive in CI round-trips.

Frontend tooling is pinned to pnpm 12.16.1 and Vite+ 1.4.0. Node is pinned in
`.node-version`. `make frontend-check` runs format, lint and type checks;
`make frontend-test` runs Vitest in both applications. Each app has its own
`pnpm-lock.yaml` and toolchain catalog. Use frozen installs; keep the catalogs
and pnpm build-script approvals under review. `vp build` is the bundler only:
use `make site` or the site's `vp run build` task for generated examples,
type checks, docs, prerendered pages and demo assembly.

Run development builds, tests and browser QA on EGGIE. Connect with
`ssh root@eggie`; do not assume an `egsox` account exists there.

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

## Marketing skill pointers

For marketing tasks, manually read [.agents/marketing-skills.md](../.agents/marketing-skills.md)
and the draft [.agents/product-marketing.md](../.agents/product-marketing.md).
Then read the selected skill's SKILL.md under
[.agents/skills/](../.agents/skills/) and its task-relevant references.
Links above are relative to this .claude/CLAUDE.md file. When reading via the
root AGENTS.md symlink, resolve them from .claude/.

.claude/skills links to the canonical .agents/skills directory for Claude's
documented project path. Pi and other agents should use the manual reading
route if their configuration does not discover these skills. Do not assume
universal auto-discovery. Context and sample copy remain drafts until approved.

## Design skill

Taste Skill (design-taste-frontend) is installed in .agents/skills/ for Codex,
Pi and OmO. See [.agents/taste-skill.md](../.agents/taste-skill.md) for provenance,
discovery and execution requirements. Preserve PRX's existing Clear Glass
design system, shared tokens and brand assets over the skill's aesthetic
defaults. Installation does not authorize UI changes or a redesign.
