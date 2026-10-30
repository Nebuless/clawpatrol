#!/usr/bin/env bash
# Builds the clickable dashboard demo into site/dist/demo/. The
# dashboard SPA is built with base /demo/; demo-mode.js is loaded
# before the bundle and answers every /api call from site/demo/fixtures
# and replays fixtures/events.jsonl as the live stream. Run after
# `pnpm run build` (it writes into the existing dist/).
set -euo pipefail
SITE="$(cd "$(dirname "$0")/.." && pwd)"
DASH="$SITE/../dashboard"
OUT="$SITE/dist/demo"

(cd "$DASH" && pnpm exec vp build --base /demo/ --outDir "$OUT" --emptyOutDir --logLevel warn)
cp "$SITE/demo/demo-mode.js" "$OUT/demo-mode.js"
cp -r "$SITE/demo/fixtures" "$OUT/fixtures"
perl -0pi -e 's{<head>}{<head>\n    <script src="/demo/demo-mode.js"></script>}' "$OUT/index.html"
grep -q 'demo-mode.js' "$OUT/index.html"
echo "Built demo -> $OUT"
