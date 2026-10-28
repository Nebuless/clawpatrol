import type { useDashboardContext } from "../lib/useDashboardContext";
import type { DashboardContext } from "../lib/dashboardContext";

export function DashboardPreferences({
  preferences: p,
}: {
  preferences: ReturnType<typeof useDashboardContext>;
}) {
  return (
    <section
      aria-label="Dashboard preferences"
      className="border-b px-4 py-3 flex flex-wrap items-end gap-4 text-sm"
    >
      <label className="flex flex-col gap-1">
        Dashboard profile
        <select
          aria-label="Dashboard profile"
          className="bg-canvas-light border p-2 min-h-11"
          value={p.context.profile ?? ""}
          disabled={p.failed}
          onChange={(e) => void p.save({ ...p.context, profile: e.target.value })}
        >
          <option value="">All integrations</option>
          {p.profiles.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </select>
      </label>
      <label className="flex flex-col gap-1">
        Dashboard theme
        <select
          aria-label="Dashboard theme"
          className="bg-canvas-light border p-2 min-h-11"
          value={p.context.theme || "light"}
          disabled={p.failed}
          onChange={(e) =>
            void p.save({ ...p.context, theme: e.target.value as DashboardContext["theme"] })
          }
        >
          <option value="light">Light</option>
          <option value="dark">Dark</option>
          <option value="system">System</option>
        </select>
      </label>
      <div className="flex-1 min-w-48">
        <p className="text-text-muted">
          Profile, theme, safe navigation and analytics filters save automatically.
        </p>
        <p role={p.failed ? "alert" : "status"} aria-live="polite">
          {p.saveStatus}
        </p>
        {p.failed && (
          <button className="underline min-h-11" onClick={() => location.reload()}>
            Reload saved preferences
          </button>
        )}
      </div>
    </section>
  );
}
