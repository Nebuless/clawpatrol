import { useCallback, useEffect, useRef, useState } from "react";
import { clearSessionCache, getState, listProfiles, type StateResp } from "./api";
import { applyTheme, contextHash, safeView, type DashboardContext } from "./dashboardContext";

async function contextRequest(method = "GET", body?: unknown) {
  const response = await fetch("/api/account/context", {
    method,
    credentials: "same-origin",
    cache: "no-store",
    ...(body
      ? { headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }
      : {}),
  });
  if (!response.ok)
    throw new Error(
      response.status === 409
        ? "Saved preferences changed or are no longer valid. Reload before saving again."
        : "Could not load or save dashboard preferences. Reload to retry.",
    );
  return response.json() as Promise<{
    context: DashboardContext;
    revision: number;
    profiles: string[];
  }>;
}
export function useDashboardContext() {
  const [state, setState] = useState<StateResp | null>(null);
  const [context, setContext] = useState<DashboardContext>({});
  const [profiles, setProfiles] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [saveStatus, setSaveStatus] = useState("");
  const [ended, setEnded] = useState(false);
  const model = useRef({
    alive: false,
    epoch: 0,
    ready: false,
    support: false,
    enabled: false,
    revision: 0,
    context: {} as DashboardContext,
    writing: false,
    queued: null as DashboardContext | null,
    failed: false,
    refreshing: false,
    sessionID: "",
  });

  const refresh = useCallback(async () => {
    const m = model.current,
      epoch = m.epoch;
    if (!m.alive || m.refreshing) return;
    m.refreshing = true;
    const current = () => m.alive && epoch === m.epoch;
    try {
      let selectedProfile = m.context.profile;
      let s = await getState(selectedProfile);
      if (m.ready && selectedProfile !== m.context.profile) return;
      if (!current()) return;
      if (!m.ready) {
        m.support = Boolean(s.support);
        m.sessionID = s.support?.session_id ?? "";
        m.enabled = s.support_available === true && !m.support;
        let c: DashboardContext = {};
        if (m.support) c = s.account_context ?? {};
        else if (m.enabled) {
          const saved = await contextRequest();
          if (!current()) return;
          c = saved.context;
          m.revision = saved.revision;
          setProfiles(saved.profiles);
        }
        if (m.support || m.enabled) {
          const names = await listProfiles();
          if (!current()) return;
          if (c.profile && !names.includes(c.profile))
            throw new Error("Saved profile is no longer available. Dashboard data is hidden.");
          setProfiles(names);
          // An explicit deep link wins. Empty hash restores the saved view.
          if (!window.location.hash || window.location.hash === "#") {
            window.location.hash = contextHash(c);
          } else if (m.support && !safeView(window.location.hash)) {
            window.location.hash = contextHash(c);
          }
          m.context = c;
          setContext(c);
          selectedProfile = c.profile;
          if (c.profile) s = await getState(c.profile);
          if (!current()) return;
        }
        m.ready = true;
      } else if (m.enabled) {
        const names = await listProfiles();
        if (!current()) return;
        if (m.context.profile && !names.includes(m.context.profile))
          throw new Error("Selected profile is no longer available. Dashboard data is hidden.");
        setProfiles(names);
      }
      if (!current() || selectedProfile !== m.context.profile) return;
      if (m.support && s.support?.session_id !== m.sessionID) {
        window.dispatchEvent(new Event("prx-support-ended"));
        return;
      }
      setState(s);
      setError("");
    } catch (e) {
      if (current()) {
        setState(null);
        setError(e instanceof Error ? e.message : "Dashboard unavailable.");
      }
    } finally {
      if (epoch === m.epoch) m.refreshing = false;
    }
  }, []);

  const save = useCallback(
    async (next: DashboardContext) => {
      const m = model.current;
      if (!m.alive || !m.ready || m.support || !m.enabled || m.failed) return;
      if (JSON.stringify(next) === JSON.stringify(m.context)) return;
      if (next.profile !== m.context.profile) setState(null);
      m.context = next;
      setContext(next);
      m.queued = next;
      if (m.writing) return;
      m.writing = true;
      const epoch = m.epoch;
      setSaveStatus("Saving preferences…");
      try {
        while (m.queued && m.alive && epoch === m.epoch) {
          const value = m.queued;
          m.queued = null;
          const result = await contextRequest("PATCH", { revision: m.revision, context: value });
          if (!m.alive || epoch !== m.epoch) return;
          m.revision = result.revision;
        }
        if (m.alive && epoch === m.epoch) {
          setSaveStatus("Preferences saved");
          void refresh();
        }
      } catch (e) {
        if (m.alive && epoch === m.epoch) {
          m.failed = true;
          m.queued = null;
          setSaveStatus(e instanceof Error ? e.message : "Preferences could not be saved.");
          setError(e instanceof Error ? e.message : "Preferences could not be saved.");
          void refresh();
        }
      } finally {
        m.writing = false;
      }
    },
    [refresh],
  );

  useEffect(() => {
    const m = model.current;
    m.alive = true;
    const end = () => {
      m.alive = false;
      m.epoch++;
      m.context = {};
      m.queued = null;
      clearSessionCache();
      setState(null);
      setContext({});
      setProfiles([]);
      setEnded(true);
    };
    const onHash = () => {
      const view = safeView(window.location.hash);
      if (view) void save({ ...m.context, ...view });
    };
    window.addEventListener("prx-support-ended", end);
    window.addEventListener("hashchange", onHash);
    void refresh().then(onHash);
    const timer = setInterval(() => void refresh(), 5000);
    return () => {
      m.alive = false;
      m.epoch++;
      m.refreshing = false;
      clearInterval(timer);
      window.removeEventListener("prx-support-ended", end);
      window.removeEventListener("hashchange", onHash);
    };
  }, [refresh, save]);
  useEffect(() => applyTheme(context.theme), [context.theme]);
  return {
    state,
    context,
    profiles,
    error,
    saveStatus,
    ended,
    refresh,
    save,
    enabled: model.current.enabled,
    failed: model.current.failed,
  };
}
