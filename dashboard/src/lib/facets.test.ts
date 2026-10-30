import { expect, test } from "vite-plus/test";
import { formatFacetValue } from "./facets";

test("facet strings preserve scalar values and empty missing fields", () => {
  expect(formatFacetValue("string", "SELECT * FROM users")).toBe("SELECT * FROM users");
  expect(formatFacetValue("int", 0)).toBe("0");
  expect(formatFacetValue("int", -42)).toBe("-42");
  expect(formatFacetValue("string", false)).toBe("false");
  expect(formatFacetValue("string", null)).toBe("");
  expect(formatFacetValue("string", undefined)).toBe("");
});

test("facet string lists keep comma formatting and omit empty entries", () => {
  expect(formatFacetValue("string_list", ["read", "", null, "write"])).toBe("read, write");
  expect(formatFacetValue("string_list", [])).toBe("");
  expect(formatFacetValue("string_list", "read")).toBe("read");
});

test("facet maps format scalar entries and reject non-map inputs", () => {
  expect(formatFacetValue("string_map", { role: "reader", limit: 10, active: false })).toBe(
    "role=reader limit=10 active=false",
  );
  expect(formatFacetValue("string_map", [])).toBe("");
  expect(formatFacetValue("string_map", "role=reader")).toBe("");
});

test("unexpected structured facet values stay readable instead of default object strings", () => {
  expect(formatFacetValue("string", { role: "reader" })).toBe('{"role":"reader"}');
  expect(formatFacetValue("string_list", [{ name: "users" }, "accounts"])).toBe(
    '{"name":"users"}, accounts',
  );
  expect(formatFacetValue("string_map", { labels: { env: "test" }, tables: ["users"] })).toBe(
    'labels={"env":"test"} tables=["users"]',
  );
  expect(formatFacetValue("future-kind", { count: 2 })).toBe('{"count":2}');
});
