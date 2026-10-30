import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { pageForPath, pages } from "../src/pages";
import { prerenderPageHtml } from "../docs-render";
import { assessmentEmail } from "../src/lib/contact";

const shell =
  '<html><head><title>PRX</title></head><body><div id="root"></div><script src="/assets/client.js"></script></body></html>';

test("assessment inquiry is a user-opened email with scope prompts", () => {
  const url = new URL(assessmentEmail);
  assert.equal(url.protocol, "mailto:");
  assert.equal(url.pathname, "pilot@nebuless.cloud");
  assert.equal(url.searchParams.get("subject"), "Discuss an on-site AI toolchain assessment");
  assert(url.searchParams.get("body")?.includes("Organization:\r\nTools and workflows to review:"));
  assert(pageForPath("/assessment/index.html"));
});

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
    assert(html.includes('href="/assessment/"'));
    assert(html.includes('href="/demo/"'));
    assert(html.includes('src="/assets/client.js"'));
    assert.equal(html.includes("SoftwareApplication"), page.path === "/");
  }
});
