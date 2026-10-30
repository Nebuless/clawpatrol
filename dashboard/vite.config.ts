// vp migrate: merged .oxlintrc.json (4fc7c6ed969d0f64844e599804661da0).
import { defineConfig, lazyPlugins } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  fmt: {
    ignorePatterns: [],
  },
  lint: {
    plugins: ["typescript", "unicorn", "oxc", "react"],
    categories: {
      correctness: "error",
    },
    rules: {
      "vite-plus/prefer-vite-plus-imports": "error",
    },
    env: {
      builtin: true,
    },
    options: {
      typeAware: true,
      typeCheck: true,
    },
    jsPlugins: [
      {
        name: "vite-plus",
        specifier: "vite-plus/oxlint-plugin",
      },
    ],
  },
  plugins: lazyPlugins(() => [react(), tailwindcss()]),
  build: {
    outDir: "dist",
    emptyOutDir: true,
    assetsDir: "_a",
    sourcemap: false,
  },
  server: {
    // Bind the dev server to loopback explicitly. Pairs with the
    // dev@local header below: the gateway trusts that header only
    // from loopback, but the proxy hop is always loopback even when
    // Vite itself binds 0.0.0.0 — so leaving host unbound would let
    // any LAN caller reach our local gateway as dev@local. Override
    // with `pnpm run dev -- --host …` when you actually want LAN
    // access (you'd also want to drop the header below first).
    host: "127.0.0.1",
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        headers: {
          // Local dev only: the gateway's tailnet gate trusts this
          // header when the request comes from loopback, which lets
          // `pnpm run dev` talk to the gateway without onboarding a
          // real wg/tailnet device.
          "Tailscale-User-Login": "dev@local",
        },
      },
      // The gateway redirects unauthenticated browsers to /__login
      // and serves the first-run / login form there. Without this
      // proxy entry, Vite would intercept the path and serve the SPA
      // shell instead — leaving you on a blank dashboard with no way
      // to log in. /__logout follows the same pattern.
      "/__login": {
        target: "http://localhost:8080",
        headers: { "Tailscale-User-Login": "dev@local" },
      },
      "/__logout": {
        target: "http://localhost:8080",
        headers: { "Tailscale-User-Login": "dev@local" },
      },
    },
  },
});
