export type SupportIdentity = {
  actor_id: string;
  actor_name: string;
  effective_id: string;
  effective_name: string;
  organization_id: string;
  installation_id: string;
  case_id: string;
  session_id: string;
  expires: number;
  saved_profile?: string;
  saved_theme?: string;
};
export type SupportCase = {
  account_id: string;
  id: string;
  technician_id: string;
  technician_name: string;
  revision: number;
  state: string;
  expires: number;
};
export type SupportStatus = {
  organization_id: string;
  installation_id: string;
  available: boolean;
  accounts: { id: string; name: string }[];
  cases: SupportCase[];
  sessions: { id: string; case_id: string; state: string; expires: number }[];
};
export function endSupportView() {
  window.dispatchEvent(new Event("prx-support-ended"));
}
export async function supportRequest<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  if (
    !(path.startsWith("/api/") || path.startsWith("/__provider/") || path === "/__support/end") ||
    new URL(path, location.origin).origin !== location.origin
  )
    throw new Error("Invalid support request target");
  const options: RequestInit = { method, cache: "no-store", credentials: "same-origin" };
  if (method !== "GET" && body !== undefined) {
    options.headers = { "Content-Type": "application/json" };
    options.body = JSON.stringify(body);
  }
  const r = await fetch(path, options);
  if (!r.ok)
    throw new Error(
      r.status === 404
        ? "Support is not enabled on this instance."
        : "Support request unavailable. Refresh assignment or check connection.",
    );
  return r.json();
}
