import { defineConfig, type Plugin, lazyPlugins } from "vite-plus";
import preact from "@preact/preset-vite";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";
import type { Doc } from "./docs-render";
import { pageMetadataForPath, pageMetadata } from "./src/page-metadata";

// Read the same pure metadata as prerendering, without bundling JSX into
// build configuration. Rewrite the title for both development and builds.
function injectTitle(): Plugin {
  return {
    name: "inject-title",
    transformIndexHtml(html, context) {
      const page = pageMetadataForPath(context.path) ?? pageMetadata[0];
      return html.replace(/<title>.*?<\/title>/, `<title>${page.title}</title>`);
    },
  };
}

function serveDocsInDev(): Plugin {
  return {
    name: "serve-docs-in-dev",
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (!req.url || !/^\/docs(?:\/|\?|$)/.test(req.url)) return next();
        try {
          const { loadDocs, renderDocPage } = await server.ssrLoadModule("/docs-render.ts");
          const docsDir = resolve(__dirname, "doc");
          const docs = loadDocs(docsDir);
          const rawPath = req.url.split("?")[0];
          const isMarkdown = rawPath.endsWith(".md");
          const path = (isMarkdown ? rawPath : rawPath.replace(/\/$/, "")) || "/docs";

          let doc: Doc | undefined;
          if (path === "/docs") {
            doc = docs[0];
          } else {
            const slug = path.replace("/docs/", "").replace(/\.md$/, "").replace(/\/$/, "");
            doc = docs.find((d: { slug: string }) => d.slug === slug);
          }
          if (!doc) return next();

          if (isMarkdown) {
            res.setHeader("Content-Type", "text/markdown; charset=utf-8");
            res.end(doc.raw);
            return;
          }

          const css = `<script type="module"
            src="/@vite/client"></script>
            <script type="module">
              import "/src/index.css";
            </script>`;
          const html = renderDocPage(doc, docs, css);
          res.setHeader("Content-Type", "text/html");
          res.end(html);
        } catch (e) {
          console.error("Docs error:", e);
          next(e);
        }
      });
    },
  };
}

/**
 * Docs HTML is server-rendered via middleware, so Vite's normal HMR doesn't
 * apply. Watch the docs template, the components it embeds, and the markdown
 * sources; trigger a full browser reload when any of them change. (CSS still
 * hot-reloads through the normal Vite path since docs.css is now imported
 * from index.css.)
 */
function reloadDocsOnChange(): Plugin {
  const docsTemplate = resolve(__dirname, "docs-render.ts");
  const docDir = resolve(__dirname, "doc");
  const sharedComponents = [
    resolve(__dirname, "src/components/Header.tsx"),
    resolve(__dirname, "src/components/Footer.tsx"),
    resolve(__dirname, "src/components/DotField.tsx"),
    resolve(__dirname, "src/components/ShadeBar.tsx"),
  ];

  return {
    name: "reload-docs-on-change",
    configureServer(server) {
      server.watcher.add(docDir);
      server.watcher.on("change", (file) => {
        const triggers =
          file === docsTemplate ||
          sharedComponents.includes(file) ||
          (file.startsWith(docDir) && file.endsWith(".md"));
        if (triggers) {
          server.ws.send({ type: "full-reload", path: "*" });
        }
      });
    },
  };
}

export default defineConfig({
  fmt: {
    ignorePatterns: ["doc/**", "public/**", "demo/fixtures/**", "src/lib/examples.ts"],
  },
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
  },
  plugins: lazyPlugins(() => [
    preact(),
    tailwindcss(),
    injectTitle(),
    serveDocsInDev(),
    reloadDocsOnChange(),
  ]),
});
