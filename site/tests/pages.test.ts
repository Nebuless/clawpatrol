import assert from "node:assert/strict";
import { test } from "node:test";
import { pageForPath, pages } from "../src/pages";
import { prerenderPageHtml } from "../docs-render";

const shell = '<html><head><title>PRX</title></head><body><div id="root"></div><script src="/assets/client.js"></script></body></html>';

test("registry selects only known pages and normalizes directory paths", () => {
  assert.equal(pageForPath("/security"), pageForPath("/security/"));
  assert.equal(pageForPath("/security/index.html"), pageForPath("/security/"));
  assert.equal(pageForPath("/missing/"), undefined);
  assert.equal(new Set(pages.map((p) => p.title)).size, pages.length);
});

test("each page prerenders its own body, metadata, and shared navigation", () => {
  for (const page of pages) {
    const html = prerenderPageHtml(shell, page);
    assert(html.includes(`<title>${page.title}</title>`));
    assert(html.includes(`rel="canonical" href="https://prx.nebuless.cloud${page.path}"`));
    assert.equal((html.match(/<h1[ >]/g) ?? []).length, 1);
    assert(html.includes('href="/pilot/"'));
    assert(html.includes('src="/assets/client.js"'));
    assert.equal(html.includes("SoftwareApplication"), page.path === "/");
  }
});
