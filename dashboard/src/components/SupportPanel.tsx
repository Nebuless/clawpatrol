import { useEffect, useState } from "react";
import { supportRequest, type SupportStatus } from "../lib/support";
import { fmtDateTime } from "../lib/format";
import { Button } from "./Button";

type AuditRow = {
  cursor: string;
  id: string;
  at: number;
  actor_id: string;
  actor_name: string;
  effective_id: string;
  effective_name: string;
  organization_id: string;
  installation_id: string;
  case_id: string;
  session_id: string;
  operation: string;
  method: string;
  decision: string;
  outcome: string;
  reason: string;
};

export function SupportPanel() {
  const [status, setStatus] = useState<SupportStatus | null>(null);
  const [error, setError] = useState("");
  const [summary, setSummary] = useState("");
  const [intent, setIntent] = useState(() => {
    const saved = sessionStorage.getItem("prx-support-case-intent");
    return saved ?? crypto.randomUUID();
  });
  const [category, setCategory] = useState("configuration");
  const [code, setCode] = useState<{ code: string; expires: number } | null>(null);
  const [busy, setBusy] = useState(false);
  const [target, setTarget] = useState("");
  const [audit, setAudit] = useState<AuditRow[]>([]);
  const [auditCursor, setAuditCursor] = useState("");
  const [auditMore, setAuditMore] = useState(false);
  const selected = status?.accounts.find((a) => a.id === target);
  async function loadAudit(cursor = "") {
    const rows = await supportRequest<AuditRow[]>(
      "/api/support/audit?limit=50&cursor=" + encodeURIComponent(cursor),
    );
    setAudit((previous) => (cursor ? [...previous, ...rows] : rows));
    setAuditCursor(rows.at(-1)?.cursor ?? cursor);
    setAuditMore(rows.length === 50);
  }
  async function exportAudit() {
    const rows: AuditRow[] = [];
    let cursor = "";
    for (;;) {
      const page = await supportRequest<AuditRow[]>(
        "/api/support/audit?limit=100&cursor=" + encodeURIComponent(cursor),
      );
      rows.push(...page);
      if (page.length < 100) break;
      const next = page.at(-1)!.cursor;
      if (!next || next === cursor) throw new Error("Audit pagination did not advance");
      cursor = next;
    }
    const url = URL.createObjectURL(
      new Blob([JSON.stringify({ schema: 1, events: rows }, null, 2)], {
        type: "application/json",
      }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "prx-support-audit.json";
    document.body.append(link);
    try {
      link.click();
    } finally {
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
  }
  function refresh() {
    return supportRequest<SupportStatus>("/api/support").then(
      (next) => {
        setStatus(next);
        setTarget((previous) =>
          next.accounts.some((a) => a.id === previous) ? previous : (next.accounts[0]?.id ?? ""),
        );
        setError("");
      },
      (e: unknown) => setError(String(e)),
    );
  }
  useEffect(() => {
    void refresh();
    const t = setInterval(refresh, 10000);
    return () => clearInterval(t);
  }, []);
  useEffect(() => {
    if (!code) return;
    const t = setTimeout(() => setCode(null), Math.max(0, code.expires * 1000 - Date.now()));
    return () => clearTimeout(t);
  }, [code]);
  async function act(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await fn();
      await refresh();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="bg-canvas border-1.5 border-navy p-4 space-y-4">
      <h2 className="text-lg font-bold">Customer-approved support</h2>
      {error && <p role="alert">{error}</p>}
      {!status && !error && <p role="status">Loading support connection…</p>}
      {status && (
        <>
          <p className="break-all">
            Organization {status.organization_id} · Installation {status.installation_id} ·{" "}
            {status.available ? "Connected" : "Offline. Local revoke remains available."}
          </p>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              void act(async () => {
                sessionStorage.setItem("prx-support-case-intent", intent);
                await supportRequest("/api/support/cases", "POST", {
                  intent_id: intent,
                  summary,
                  category,
                });
                sessionStorage.removeItem("prx-support-case-intent");
                setSummary("");
                setIntent(crypto.randomUUID());
              });
            }}
          >
            <label className="block">
              Case summary{" "}
              <input
                className="block w-full border p-2"
                required
                maxLength={240}
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
                aria-describedby="support-summary-help"
              />
            </label>
            <p id="support-summary-help">
              Describe the issue without secrets, tokens, passwords or SUPPORT codes.
            </p>
            <label className="block">
              Category{" "}
              <select
                className="border p-2"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
              >
                <option value="configuration">Configuration</option>
                <option value="connection">Connection</option>
                <option value="other">Other</option>
              </select>
            </label>
            <Button type="submit" disabled={busy || !status.available || !selected}>
              Submit support case
            </Button>
          </form>
          {status.accounts.length > 1 ? (
            <label className="block">
              Eligible account
              <select
                className="border p-2 max-w-full"
                value={target}
                onChange={(e) => {
                  setTarget(e.target.value);
                  setCode(null);
                }}
              >
                {status.accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} · {a.id}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <p>
              {selected
                ? "Eligible account: " + selected.name
                : "No eligible account. Support approval unavailable."}
            </p>
          )}
          {status.cases.length === 0 && (
            <p>No support cases. Submit a case before requesting access.</p>
          )}
          {status.cases.map((c) => (
            <div key={c.id} className="space-y-2 border-t pt-3 break-words">
              <h3 className="font-bold">Case {c.id}</h3>
              <p>
                {c.state === "assigned"
                  ? "Assigned to " + c.technician_name
                  : "Waiting for technician assignment"}
              </p>
              {c.state === "assigned" && (
                <>
                  <p>
                    SUPPORT code, not your login or MFA code. Read only to {c.technician_name} for
                    this case. Access lasts at most 15 minutes. A new code ends prior access.
                  </p>
                  <Button
                    disabled={
                      busy || !status.available || !selected || c.account_id !== selected.id
                    }
                    onClick={() =>
                      void act(async () => {
                        setCode(null);
                        setCode(
                          await supportRequest("/api/support/grants", "POST", {
                            case_id: c.id,
                            target_account_id: selected?.id,
                            assignment_revision: c.revision,
                          }),
                        );
                      })
                    }
                  >
                    Issue one-time SUPPORT code for {selected?.name ?? "unavailable account"}
                  </Button>
                </>
              )}
            </div>
          ))}
          {code && (
            <div role="status" className="space-y-2">
              <label className="block">
                One-time SUPPORT code{" "}
                <input className="block w-full border p-2 font-mono" readOnly value={code.code} />
              </label>
              <p>
                Expires {fmtDateTime(new Date(code.expires * 1000))}. Shown once. Do not save in the
                case.
              </p>
              <Button onClick={() => setCode(null)}>Hide code</Button>
            </div>
          )}
          {(status.sessions ?? []).map((s) => (
            <div key={s.id}>
              <p>
                Case {s.case_id} · {s.state} · Ends {fmtDateTime(new Date(s.expires * 1000))}
              </p>
              <Button
                disabled={busy}
                onClick={() =>
                  void act(async () => {
                    await supportRequest("/api/support/revoke", "POST", { id: s.id });
                    setCode(null);
                  })
                }
              >
                Revoke access
              </Button>
            </div>
          ))}
          <div className="flex flex-wrap gap-2">
            <Button disabled={busy} onClick={() => void act(() => loadAudit())}>
              Load local audit
            </Button>
            <Button disabled={busy} onClick={() => void act(exportAudit)}>
              Export local audit JSON
            </Button>
          </div>
          <ul aria-label="Local support audit" className="space-y-3 break-words">
            {audit.map((a) => (
              <li key={a.id} className="border-t pt-2">
                <p>
                  {fmtDateTime(new Date(a.at * 1000))} · {a.operation} · {a.decision} · {a.outcome}
                </p>
                <p>
                  Actor: {a.actor_name || "Unresolved actor"} ({a.actor_id || "unresolved"})
                </p>
                <p>
                  Effective customer: {a.effective_name || "Not resolved"} (
                  {a.effective_id || "unresolved"})
                </p>
                <p>
                  Organization {a.organization_id} · Installation {a.installation_id}
                </p>
                <p>
                  Case {a.case_id || "none"} · Session {a.session_id || "none"} · {a.method} ·{" "}
                  {a.reason}
                </p>
              </li>
            ))}
          </ul>
          {auditMore && (
            <Button disabled={busy} onClick={() => void act(() => loadAudit(auditCursor))}>
              Load more audit
            </Button>
          )}
        </>
      )}
    </section>
  );
}
