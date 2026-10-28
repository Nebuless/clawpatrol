import { useEffect, useState } from "react";
import { supportRequest } from "../lib/support";
import { Button } from "./Button";
import { Main } from "./Main";
import { PageTitle } from "./PageTitle";
type Org = { id: string; name: string; enabled: boolean };
type Installation = Org & { hosting: string; enabled: boolean };
type Tech = Org & { administrator: boolean };
type Case = { id: string; state: string; technician_id: string; technician_name: string };
export function ProviderPage() {
  const [actor, setActor] = useState<Tech | null>(null);
  const [orgs, setOrgs] = useState<Org[]>([]);
  const [org, setOrg] = useState("");
  const [installations, setInstallations] = useState<Installation[]>([]);
  const [installation, setInstallation] = useState("");
  const [health, setHealth] = useState<{ connector: string; registry: string } | null>(null);
  const [cases, setCases] = useState<Case[]>([]);
  const [techs, setTechs] = useState<Tech[]>([]);
  const [tech, setTech] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const query =
    "?organization=" +
    encodeURIComponent(org) +
    "&installation=" +
    encodeURIComponent(installation);
  useEffect(() => {
    let live = true;
    Promise.all([
      supportRequest<Tech>("/__provider/session"),
      supportRequest<Org[]>("/__provider/organizations"),
      supportRequest<Tech[]>("/__provider/technicians"),
    ])
      .then(([a, o, t]) => {
        if (live) {
          setActor(a);
          setOrgs(o);
          setTechs(t);
        }
      })
      .catch((e) => setError(String(e)));
    return () => {
      live = false;
    };
  }, []);
  useEffect(() => {
    setHealth(null);
    setCode("");
    setInstallations([]);
    setInstallation("");
    setCases([]);
    if (!org) return;
    let live = true;
    supportRequest<Installation[]>(
      "/__provider/installations?organization=" + encodeURIComponent(org),
    )
      .then((v) => {
        if (live) setInstallations(v);
      })
      .catch((e) => setError(String(e)));
    return () => {
      live = false;
    };
  }, [org]);
  async function refresh() {
    if (installation) {
      try {
        setHealth(await supportRequest("/__provider/installation" + query));
      } catch (error) {
        setHealth({ connector: "Unavailable", registry: "Unknown" });
        setCases([]);
        throw error;
      }
      const chosen = installations.find((i) => i.id === installation);
      setCases(
        chosen?.enabled && orgs.find((o) => o.id === org)?.enabled
          ? await supportRequest<Case[]>("/__provider/cases" + query)
          : [],
      );
    }
  }
  useEffect(() => {
    setCode("");
    setCases([]);
    setHealth(null);
    if (!installation) return;
    let live = true;
    supportRequest<{ connector: string; registry: string }>("/__provider/installation" + query)
      .then(async (h) => {
        if (!live) return;
        setHealth(h);
        if (
          !installations.find((i) => i.id === installation)?.enabled ||
          !orgs.find((o) => o.id === org)?.enabled
        )
          return;
        try {
          const c = await supportRequest<Case[]>("/__provider/cases" + query);
          if (live) setCases(c);
        } catch (e) {
          if (live) setError(String(e));
        }
      })
      .catch((e) => {
        if (live) {
          setHealth({ connector: "Unavailable", registry: "Unknown" });
          setError(String(e));
        }
      });
    return () => {
      live = false;
    };
  }, [query, installation, installations, orgs, org]);
  async function act(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Main>
      <PageTitle trail={[{ label: "Provider" }]} />
      <section className="bg-canvas border-1.5 border-navy p-4 space-y-4">
        <h1 className="text-lg font-bold">PRX Provider</h1>
        <p>
          Technician: {actor?.name ?? "Loading…"}. Organization management does not grant customer
          dashboard access.
        </p>
        {error && <p role="alert">{error}</p>}
        <label className="block">
          Organization{" "}
          <select
            className="border p-2 max-w-full"
            value={org}
            disabled={busy}
            onChange={(e) => setOrg(e.target.value)}
          >
            <option value="">Choose organization</option>
            {orgs.map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          Installation{" "}
          <select
            className="border p-2 max-w-full"
            value={installation}
            disabled={busy}
            onChange={(e) => setInstallation(e.target.value)}
          >
            <option value="">Choose installation</option>
            {installations.map((i) => (
              <option key={i.id} value={i.id}>
                {i.name} · {i.hosting} · {i.enabled ? "enabled" : "disabled"}
              </option>
            ))}
          </select>
        </label>
        {actor?.administrator && (
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              const data = new FormData(e.currentTarget);
              void act(async () => {
                await supportRequest("/__provider/organization", "POST", {
                  id: data.get("id"),
                  name: data.get("name"),
                  enabled: true,
                });
                setOrgs(await supportRequest<Org[]>("/__provider/organizations"));
              });
            }}
          >
            <h2 className="font-bold">Create organization</h2>
            <label className="block">
              Organization ID{" "}
              <input name="id" required maxLength={128} className="border p-2 w-full" />
            </label>
            <label className="block">
              Organization name{" "}
              <input name="name" required maxLength={240} className="border p-2 w-full" />
            </label>
            <Button disabled={busy} type="submit">
              Create organization
            </Button>
          </form>
        )}
        {org && (
          <section aria-label="Organization management" className="space-y-2 break-words">
            <h2 className="font-bold">Organization management</h2>
            <p>
              {orgs.find((o) => o.id === org)?.name} · {org} ·{" "}
              {orgs.find((o) => o.id === org)?.enabled ? "Enabled" : "Disabled"}
            </p>
            {actor?.administrator && (
              <>
                <Button
                  disabled={busy}
                  onClick={() =>
                    void act(async () => {
                      const o = orgs.find((o) => o.id === org);
                      if (!o) return;
                      await supportRequest(
                        "/__provider/organization?organization=" + encodeURIComponent(org),
                        "PATCH",
                        { ...o, enabled: !o.enabled },
                      );
                      setOrgs(await supportRequest<Org[]>("/__provider/organizations"));
                    })
                  }
                >
                  Toggle organization enabled
                </Button>
                <form
                  className="space-y-2"
                  onSubmit={(e) => {
                    e.preventDefault();
                    const data = new FormData(e.currentTarget);
                    void act(async () => {
                      await supportRequest(
                        "/__provider/organization-assign?organization=" + encodeURIComponent(org),
                        "POST",
                        { Technician: data.get("technician") },
                      );
                    });
                  }}
                >
                  <label className="block">
                    Organization technician{" "}
                    <select name="technician" required className="border p-2 max-w-full">
                      {techs.map((t) => (
                        <option key={t.id} value={t.id}>
                          {t.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <Button disabled={busy} type="submit">
                    Assign organization technician
                  </Button>
                </form>
              </>
            )}
          </section>
        )}
        {actor?.administrator && org && (
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              const data = new FormData(e.currentTarget);
              void act(async () => {
                await supportRequest("/__provider/register", "POST", {
                  id: data.get("id"),
                  organization_id: org,
                  name: data.get("name"),
                  hosting: data.get("hosting"),
                  enabled: true,
                });
                setInstallations(
                  await supportRequest<Installation[]>(
                    "/__provider/installations?organization=" + encodeURIComponent(org),
                  ),
                );
              });
            }}
          >
            <h2 className="font-bold">Register existing installation</h2>
            <label className="block">
              Gateway installation ID <input name="id" required className="border p-2 w-full" />
            </label>
            <label className="block">
              Installation name <input name="name" required className="border p-2 w-full" />
            </label>
            <label className="block">
              Hosting{" "}
              <select name="hosting" className="border p-2">
                <option value="customer">Customer-hosted</option>
                <option value="nebuless">Nebuless-hosted</option>
              </select>
            </label>
            <Button disabled={busy} type="submit">
              Register installation
            </Button>
            <p>
              Registration is metadata. It does not provision infrastructure or enroll a connector.
            </p>
          </form>
        )}
        {installation && (
          <section aria-label="Installation management" className="space-y-3 break-words">
            <h2 className="font-bold">Installation management</h2>
            <p>Installation ID: {installation}</p>
            <p>
              {installations.find((i) => i.id === installation)?.name} ·{" "}
              {installations.find((i) => i.id === installation)?.hosting}-hosted ·{" "}
              {installations.find((i) => i.id === installation)?.enabled ? "Enabled" : "Disabled"}
            </p>
            <p>
              Connector: {health?.connector ?? "Checking…"} · Client registry:{" "}
              {health?.registry ?? "Unknown"}
            </p>
            <p>
              Health contains connection and sync state only. No customer data or registry
              credentials.
            </p>
          </section>
        )}
        {installation && (
          <>
            <Button disabled={busy} onClick={() => void act(refresh)}>
              Refresh installation and cases
            </Button>
            {actor?.administrator && (
              <Button
                disabled={busy}
                onClick={() =>
                  void act(async () => {
                    const i = installations.find((i) => i.id === installation);
                    if (!i) return;
                    await supportRequest("/__provider/installation" + query, "POST", {
                      Name: i.name,
                      Enabled: !i.enabled,
                    });
                    setInstallations(
                      await supportRequest<Installation[]>(
                        "/__provider/installations?organization=" + encodeURIComponent(org),
                      ),
                    );
                  })
                }
              >
                Toggle installation enabled
              </Button>
            )}
            <label className="block">
              Assigned technician{" "}
              <select
                value={tech}
                onChange={(e) => setTech(e.target.value)}
                className="border p-2 max-w-full"
              >
                <option value="">Choose technician</option>
                {techs.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              Customer phone SUPPORT code{" "}
              <input
                autoComplete="off"
                spellCheck={false}
                className="border p-2 w-full font-mono"
                value={code}
                onChange={(e) => setCode(e.target.value)}
              />
            </label>
            {cases.map((c) => (
              <div key={c.id} className="border-t pt-3 space-y-2 break-all">
                <h2 className="font-bold">Case {c.id}</h2>
                <p>
                  {c.state} · {c.technician_name || "Unassigned"}
                </p>
                {actor?.administrator && (
                  <Button
                    disabled={busy || !tech}
                    onClick={() =>
                      void act(async () => {
                        await supportRequest("/__provider/assign" + query, "POST", {
                          Case: c.id,
                          Technician: tech,
                        });
                        await refresh();
                      })
                    }
                  >
                    Assign technician
                  </Button>
                )}
                <form method="post" action={"/__provider/view-as" + query}>
                  <input type="hidden" name="case" value={c.id} />
                  <input type="hidden" name="code" value={code} />
                  <Button type="submit" disabled={busy || !code || c.technician_id !== actor?.id}>
                    View customer dashboard
                  </Button>
                </form>
              </div>
            ))}
            <p>
              View-as always requires customer approval and recent technician MFA, including
              Nebuless-hosted installations.
            </p>
          </>
        )}
        <form action="/__provider/logout" method="post">
          <Button type="submit">Log out of Provider</Button>
        </form>
      </section>
    </Main>
  );
}
