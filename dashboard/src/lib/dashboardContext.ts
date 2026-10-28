export type DashboardContext = {
  profile?: string;
  theme?: "light" | "dark" | "system";
  route?: { page?: string; device_ip?: string; request_id?: string };
  filters?: { range?: string; color?: string; scale?: string; device_ip?: string; host?: string };
};
export const ranges = ["1m", "5m", "15m", "30m", "1h", "6h", "24h"];
export function validIP(ip: string): boolean {
  if (!ip || ip.length > 45 || /[%/\s?#]/.test(ip)) return false;
  if (!ip.includes(":"))
    return (
      /^(0|[1-9]\d{0,2})(\.(0|[1-9]\d{0,2})){3}$/.test(ip) &&
      ip.split(".").every((v) => Number(v) <= 255)
    );
  try {
    return new URL("http://[" + ip + "]").hostname.startsWith("[");
  } catch {
    return false;
  }
}
export function safeView(hash: string): Pick<DashboardContext, "route" | "filters"> | null {
  const [path, query = ""] = hash.replace(/^#/, "").split("?");
  let parts: string[];
  try {
    parts = (path || "/").split("/").slice(1).map(decodeURIComponent);
  } catch {
    return null;
  }
  const page = parts[0] || "main";
  if (
    ![
      "main",
      "devices",
      "device",
      "analytics",
      "request",
      "settings",
      "plugins",
      "account",
    ].includes(page)
  )
    return null;
  const route: NonNullable<DashboardContext["route"]> = { page };
  if (parts.length > 2) return null;
  if (parts[1]) {
    if ((page === "device" || page === "analytics") && validIP(parts[1]))
      route.device_ip = parts[1];
    else if (page === "request" && /^[A-Za-z0-9_-]{1,128}$/.test(parts[1]))
      route.request_id = parts[1];
    else return null;
  } else if (page === "device" || page === "request") return null;
  const filters: NonNullable<DashboardContext["filters"]> = {};
  if (page === "analytics") {
    const q = new URLSearchParams(query);
    for (const [key, allowed] of Object.entries({
      range: ranges,
      color: ["host", "device", "status"],
      scale: ["log", "linear"],
    })) {
      const value = q.get(key);
      if (value && q.getAll(key).length === 1 && allowed.includes(value))
        filters[key as "range" | "color" | "scale"] = value;
    }
    const ip = q.get("device"),
      host = q.get("host");
    if (ip && q.getAll("device").length === 1 && validIP(ip)) filters.device_ip = ip;
    if (
      host &&
      q.getAll("host").length === 1 &&
      host.length <= 253 &&
      /^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$/.test(host)
    )
      filters.host = host;
  }
  return { route, ...(page === "analytics" ? { filters } : {}) };
}
export function contextHash(c: DashboardContext): string {
  const r = c.route;
  if (!r?.page || r.page === "main") return "#/";
  let hash = "#/" + r.page;
  if (r.device_ip) hash += "/" + encodeURIComponent(r.device_ip);
  if (r.request_id) hash += "/" + encodeURIComponent(r.request_id);
  if (r.page === "analytics") {
    const q = new URLSearchParams();
    for (const [key, value] of Object.entries(c.filters ?? {}))
      if (value) q.set(key === "device_ip" ? "device" : key, value);
    if (q.size) hash += "?" + q;
  }
  return hash;
}
export function applyTheme(theme: DashboardContext["theme"]) {
  const media = matchMedia("(prefers-color-scheme: dark)");
  const apply = () => {
    document.documentElement.dataset.theme =
      theme === "dark" || (theme === "system" && media.matches) ? "dark" : "light";
  };
  apply();
  media.addEventListener("change", apply);
  return () => {
    media.removeEventListener("change", apply);
    document.documentElement.removeAttribute("data-theme");
  };
}
