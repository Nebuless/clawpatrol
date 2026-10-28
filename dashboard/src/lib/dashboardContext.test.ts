import { contextHash, safeView, validIP } from "./dashboardContext";
function test(_name: string, run: () => void) {
  run();
}
function assert(value: unknown, message: string) {
  if (!value) throw new Error(message);
}
test("saved view roundtrip preserves allowed analytics controls only", () => {
  const view = safeView(
    "#/analytics/fd7a%3A115c%3Aa1e0%3A%3A1?range=6h&color=host&scale=linear&device=10.55.0.2&host=public.example&code=SUPPORT-secret&token=secret",
  );
  assert(view?.route?.device_ip === "fd7a:115c:a1e0::1", "IPv6 route");
  assert(view?.filters?.host === "public.example", "host filter");
  const encoded = contextHash(view!);
  assert(!encoded.includes("secret"), "secret query retained");
  assert(JSON.stringify(safeView(encoded)) === JSON.stringify(view), "roundtrip changed filters");
});
test("unsafe routes and malformed selectors cannot become saved preferences", () => {
  for (const hash of [
    "#/onboard/code",
    "#/login",
    "#/support/secret",
    "https://foreign.example/",
    "#/device/not-an-ip",
    "#/device/999.1.1.1",
    "#/request/..",
    "#/request/a%2Fb",
    "#/analytics/%zz",
  ])
    assert(safeView(hash) === null, hash);
  const filters = safeView(
    "#/analytics?range=1h&range=24h&scale=bad&host=https%3A%2F%2Fsecret&device=bad",
  )?.filters;
  assert(JSON.stringify(filters) === "{}", "invalid selectors retained");
  assert(
    validIP("::1") && validIP("10.55.0.2") && !validIP("fe80::1%eth0") && !validIP("010.1.1.1"),
    "IP constraints",
  );
});
