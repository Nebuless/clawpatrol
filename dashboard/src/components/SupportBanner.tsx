import { useState } from "react";
import { endSupportView, type SupportIdentity } from "../lib/support";
import { fmtDateTime } from "../lib/format";
import { Button } from "./Button";
export function SupportBanner({ identity }: { identity: SupportIdentity }) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <aside
      aria-label="Support session"
      className="bg-butter-100 border-b border-butter-300 p-4 flex flex-col sm:flex-row flex-wrap items-start sm:items-center gap-3"
    >
      <div className="flex-1 min-w-0 break-words">
        <strong>Support session</strong>
        <p>
          {identity.actor_name} is viewing as {identity.effective_name} · {identity.organization_id}
        </p>
        <p>
          Case {identity.case_id} · Ends {fmtDateTime(new Date(identity.expires * 1000))}
        </p>
        <p>
          Saved profile: {identity.saved_profile || "Default"} · Saved theme:{" "}
          {identity.saved_theme || "System"}
        </p>
        <p>Secrets, enrollment, approvals and arbitrary HCL changes unavailable during support.</p>
      </div>
      <Button
        className="min-h-11"
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          try {
            const r = await fetch("/__support/end", {
              method: "POST",
              credentials: "same-origin",
              cache: "no-store",
            });
            if (!r.ok && r.status !== 401)
              throw new Error("End failed. Retry or ask customer to revoke access.");
            endSupportView();
          } catch (e) {
            setError(String(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        End support session
      </Button>
      {error && <p role="alert">{error}</p>}
    </aside>
  );
}
