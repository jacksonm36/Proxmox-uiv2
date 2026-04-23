import { useEffect, useState, type FormEvent } from "react";
import { Link, NavLink, Route, Routes } from "react-router-dom";
import {
  fetchMe,
  getVMs,
  getTemplates,
  getAudit,
  getTFWorkspaces,
  getPveConnection,
  postPveConnection,
  postPveConsole,
  getAdminEnv,
  patchAdminEnv,
  getOrg,
  setOrg,
  initOrgFromMe,
  type AdminEnvGet,
} from "./api";
import {
  IconActivity,
  IconBell,
  IconChevronRight,
  IconCompute,
  IconImages,
  IconLogo,
  IconProject,
  IconSearch,
  IconSettings,
  IconSunMoon,
  IconTerraform,
} from "./icons";

type Me = Awaited<ReturnType<typeof fetchMe>>;

function canConfigurePve(me: Me) {
  return me.user.isPlatformAdmin || me.orgRole === "org_admin";
}

type QuickStats = {
  vms: number | null;
  templates: number | null;
  workspaces: number | null;
  audit: number | null;
};

export function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [quickStats, setQuickStats] = useState<QuickStats | null>(null);

  useEffect(() => {
    fetchMe()
      .then((d) => {
        initOrgFromMe(d);
        setMe(d);
        setErr(null);
      })
      .catch((e) => {
        setErr(String(e));
        setMe(null);
      });
  }, []);

  useEffect(() => {
    if (!me) {
      setQuickStats(null);
      return;
    }
    let cancelled = false;
    (async () => {
      const [a, b, c, d] = await Promise.allSettled([
        getVMs(),
        getTemplates(),
        getTFWorkspaces(),
        getAudit(),
      ]);
      if (cancelled) return;
      setQuickStats({
        vms: a.status === "fulfilled" ? a.value.vms.length : null,
        templates: b.status === "fulfilled" ? b.value.templates.length : null,
        workspaces: c.status === "fulfilled" ? c.value.workspaces.length : null,
        audit: d.status === "fulfilled" ? d.value.items.length : null,
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [me?.user.id]);

  if (err && !me) {
    return (
      <div className="shell shell--light">
        <p className="shell__msg warn">Not signed in. In development, <a href="/api/v1/auth/dev">obtain a session</a> then refresh.</p>
      </div>
    );
  }
  if (!me) {
    return <div className="shell shell--light muted">Loading…</div>;
  }

  return (
    <div className="app">
      <SideNav me={me} />
      <div className="app__col">
        <AppTopBar me={me} />
        <main className="app__main">
        <Routes>
          <Route path="/" element={<Dashboard me={me} stats={quickStats} />} />
          <Route path="/compute" element={<ComputePage />} />
          <Route path="/images" element={<ImagesPage />} />
          <Route path="/terraform" element={<TerraformPage />} />
          <Route path="/activity" element={<ActivityPage />} />
          <Route path="/settings" element={<SettingsPage me={me} />} />
          <Route path="/admin/env" element={<AdminEnvPage me={me} />} />
        </Routes>
        </main>
      </div>
    </div>
  );
}

function userInitial(email: string) {
  const c = email.trim().charAt(0).toUpperCase();
  return c || "?";
}

function AppTopBar({ me }: { me: Me }) {
  const orgLabel =
    me.orgs.length === 0
      ? "—"
      : (me.orgs.find((o) => o.orgId === (getOrg() || (me.orgIdHeader as string)))?.org.name ??
        me.orgs[0].org.name);

  return (
    <header className="apphead">
      <div className="apphead__search" role="search">
        <IconSearch />
        <input type="search" name="q" placeholder="Search VMs, orgs, records…" autoComplete="off" />
      </div>
      <div className="apphead__end">
        <span className="apphead__org" title="Active organization">
          {orgLabel}
        </span>
        {me.user.isPlatformAdmin && <span className="vz-pill">Platform</span>}
        <OrgPicker me={me} />
        <button type="button" className="apphead__icon" title="Notifications" aria-label="Notifications">
          <IconBell />
          <span className="apphead__notify">0</span>
        </button>
        <button type="button" className="apphead__icon" title="Display (placeholder)" aria-label="Theme">
          <IconSunMoon />
        </button>
        <div className="apphead__user" title={me.user.email} aria-label="Signed in user">
          {userInitial(me.user.email)}
        </div>
      </div>
    </header>
  );
}

function SideNav({ me }: { me: Me }) {
  return (
    <aside className="sidenav" aria-label="Main navigation">
      <div className="sidenav__brand">
        <IconLogo />
        <div className="sidenav__wordmark">
          <span className="sidenav__name">cloudmanager</span>
          <span className="sidenav__sub">Proxmox control</span>
        </div>
      </div>
      <label className="sidenav__search">
        <IconSearch />
        <input type="search" placeholder="Type to search" autoComplete="off" />
      </label>
      <nav className="sidenav__list">
        <NavLink
          to="/"
          end
          className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
        >
          <IconProject />
          <span>Dashboard</span>
          <IconChevronRight className="sidenav__chev" />
        </NavLink>
        <NavLink
          to="/compute"
          className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
        >
          <IconCompute />
          <span>Compute</span>
          <IconChevronRight className="sidenav__chev" />
        </NavLink>
        <NavLink
          to="/images"
          className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
        >
          <IconImages />
          <span>Images</span>
          <IconChevronRight className="sidenav__chev" />
        </NavLink>
        <NavLink
          to="/terraform"
          className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
        >
          <IconTerraform />
          <span>Terraform</span>
          <IconChevronRight className="sidenav__chev" />
        </NavLink>
        <NavLink
          to="/activity"
          className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
        >
          <IconActivity />
          <span>Activity</span>
          <IconChevronRight className="sidenav__chev" />
        </NavLink>
        {canConfigurePve(me) && (
          <NavLink
            to="/settings"
            className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
          >
            <IconSettings />
            <span>Proxmox env</span>
            <IconChevronRight className="sidenav__chev" />
          </NavLink>
        )}
        {me.user.isPlatformAdmin && (
          <NavLink
            to="/admin/env"
            className={({ isActive }) => (isActive ? "sidenav__item is-active" : "sidenav__item")}
          >
            <IconSettings />
            <span>Server .env</span>
            <IconChevronRight className="sidenav__chev" />
          </NavLink>
        )}
      </nav>
      <div className="sidenav__foot">
        {me.user.isPlatformAdmin && <span className="sidenav__footnote">Platform access</span>}
      </div>
    </aside>
  );
}

function OrgPicker({ me }: { me: Me }) {
  if (me.user.isPlatformAdmin && me.orgs.length === 0) {
    return <span className="org muted">(platform)</span>;
  }
  if (me.orgs.length === 0) {
    return <span className="org warn">no org</span>;
  }
  const orgId = getOrg() || (me.orgIdHeader as string) || me.orgs[0].org.id;
  return (
    <label className="org org--header">
      <span className="org__label">Org</span>
      <select
        className="org__select"
        value={orgId}
        onChange={(e) => {
          setOrg(e.target.value);
          window.location.reload();
        }}
      >
        {me.orgs.map((o) => (
          <option key={o.orgId} value={o.orgId}>
            {o.org.name} ({o.role})
          </option>
        ))}
      </select>
    </label>
  );
}

function DonutStatCard({
  title,
  value,
  colors,
  legend,
}: {
  title: string;
  value: string | number;
  colors: [string, string, string];
  legend: [string, string, string];
}) {
  const [a, b, c] = colors;
  return (
    <div className="vz-card vz-stat">
      <h3 className="vz-stat__title">{title}</h3>
      <div className="vz-stat__row">
        <div
          className="vz-donut"
          style={{
            background: `conic-gradient(${a} 0deg 120deg, ${b} 120deg 240deg, ${c} 240deg 360deg)`,
          }}
        >
          <div className="vz-donut__hole" />
          <div className="vz-donut__num" aria-label={`${title} total`}>
            {value}
          </div>
        </div>
        <ul className="vz-legend">
          {legend.map((label, i) => (
            <li key={`${title}-${i}`}>
              <span
                className="vz-legend__dot"
                style={{ background: i === 0 ? a : i === 1 ? b : c }}
              />
              <span className="vz-legend__label">{label}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

function Dashboard({ me, stats }: { me: Me; stats: QuickStats | null }) {
  const s = stats;
  return (
    <div className="page page--home">
      <div className="page__bar">
        <h1 className="page__title">Dashboard</h1>
        <p className="lede lede--inline">
          Overview for{" "}
          {me.orgs.length === 0
            ? "—"
            : (me.orgs.find((o) => o.orgId === (getOrg() || (me.orgIdHeader as string)))?.org.name ??
              me.orgs[0].org.name)}
        </p>
      </div>

      <section className="statgrid" aria-label="At a glance">
        <DonutStatCard
          title="Instances"
          value={s ? (s.vms ?? "—") : "…"}
          colors={["#2563eb", "#2dd4bf", "#a78bfa"]}
          legend={["In API scope", "Nodes reporting", "Other"]}
        />
        <DonutStatCard
          title="Images"
          value={s ? (s.templates ?? "—") : "…"}
          colors={["#7c3aed", "#0ea5e9", "#22c55e"]}
          legend={["Template catalog", "Per node", "—"]}
        />
        <DonutStatCard
          title="Terraform"
          value={s ? (s.workspaces ?? "—") : "…"}
          colors={["#0ea5e9", "#f472b6", "#6366f1"]}
          legend={["Workspaces", "Queued", "—"]}
        />
        <DonutStatCard
          title="Audit"
          value={s ? (s.audit ?? "—") : "…"}
          colors={["#10b981", "#f59e0b", "#8b5cf6"]}
          legend={["Events (loaded)", "Recent window", "—"]}
        />
      </section>

      <h2 className="section__title">Where to next</h2>
      <div className="cards">
        <Link className="card" to="/compute">
          <span className="card__icon" aria-hidden>
            <IconCompute />
          </span>
          <div className="card__body">
            <h3>Instances</h3>
            <p>View and control VMs your token can access.</p>
          </div>
          <IconChevronRight />
        </Link>
        <Link className="card" to="/images">
          <span className="card__icon" aria-hidden>
            <IconImages />
          </span>
          <div className="card__body">
            <h3>Template catalog</h3>
            <p>Org-approved templates for new workloads.</p>
          </div>
          <IconChevronRight />
        </Link>
        <Link className="card" to="/terraform">
          <span className="card__icon" aria-hidden>
            <IconTerraform />
          </span>
          <div className="card__body">
            <h3>Terraform</h3>
            <p>Workspaces, plans, and applies (worker runs applies).</p>
          </div>
          <IconChevronRight />
        </Link>
        <Link className="card" to="/activity">
          <span className="card__icon" aria-hidden>
            <IconActivity />
          </span>
          <div className="card__body">
            <h3>Activity</h3>
            <p>Audit log for the selected organization.</p>
          </div>
          <IconChevronRight />
        </Link>
      </div>
    </div>
  );
}

const ENV_LABELS: Record<string, string> = {
  CM_HTTP_ADDR: "HTTP listen address (bind)",
  CM_DATABASE_URL: "PostgreSQL URL (paste full URL; masked when saved)",
  CM_SESSION_SECRET: "Session secret (min 32 characters)",
  CM_ENCRYPTION_KEY: "Encryption key (64 hex characters)",
  CM_REDIS_ADDR: "Redis (optional, host:port)",
  CM_BASE_URL: "Public app URL / IP (users and callbacks)",
  CM_CORS_ORIGINS: "CORS allowed origins (comma-separated)",
  CM_TRUSTED_PROXIES: "Reverse proxy CIDRs (comma-separated)",
  CM_TERRAFORM_PATH: "Terraform binary name or path",
  CM_WORKDIR: "Application work directory on disk",
  CM_WEB_ROOT: "Built SPA static files directory (optional)",
  CM_DISABLE_OIDC: "Disable OIDC (1 or 0)",
  CM_DEV_BOOTSTRAP: "Dev database bootstrap (1 or 0)",
  CM_OIDC_ISSUER: "OIDC issuer (realm URL)",
  CM_OIDC_CLIENT_ID: "OIDC client id",
  CM_OIDC_CLIENT_SECRET: "OIDC client secret",
  CM_OIDC_REDIRECT_URL: "OIDC redirect URL",
  CM_DEV_BEARER: "Dev / automation bearer token (optional)",
  CM_DEV_USER_EMAIL: "Dev bootstrap user email",
  CM_WORKER_CONCURRENCY: "Background worker job concurrency",
};

const SERVER_SECRET_KEYS = new Set([
  "CM_SESSION_SECRET",
  "CM_ENCRYPTION_KEY",
  "CM_OIDC_CLIENT_SECRET",
  "CM_DEV_BEARER",
]);

function AdminEnvPage({ me }: { me: Me }) {
  const [data, setData] = useState<AdminEnvGet | null>(null);
  const [form, setForm] = useState<Record<string, string>>({});
  const [initial, setInitial] = useState<Record<string, string>>({});
  const [err, setErr] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setErr(null);
    getAdminEnv()
      .then((d) => {
        setData(d);
        const f: Record<string, string> = {};
        for (const k of d.keys) {
          f[k] = d.values[k] ?? "";
        }
        setForm(f);
        setInitial({ ...f });
      })
      .catch((e) => setErr(e instanceof Error ? e.message : String(e)));
  }, []);

  if (!me.user.isPlatformAdmin) {
    return (
      <div className="page">
        <p className="warn">Only platform administrators can edit the server <code>managed.env</code> file.</p>
      </div>
    );
  }

  async function onAdminEnvSubmit(e: FormEvent) {
    e.preventDefault();
    if (!data) return;
    setErr(null);
    setOk(null);
    const patch: Record<string, string> = {};
    for (const k of data.keys) {
      const cur = form[k] ?? "";
      const ini = initial[k] ?? "";
      if (SERVER_SECRET_KEYS.has(k)) {
        if (cur.trim() !== "") {
          patch[k] = cur;
        }
        continue;
      }
      if (k === "CM_DATABASE_URL" && cur.includes("***") && cur === ini) {
        continue;
      }
      if (cur !== ini) {
        patch[k] = cur;
      }
    }
    if (Object.keys(patch).length === 0) {
      setOk("No changes to save.");
      return;
    }
    setSaving(true);
    try {
      const r = await patchAdminEnv(patch);
      setOk(r.notice + (r.relogin ? " " + r.relogin : "") + " — file: " + (r.path || ""));
      const d = await getAdminEnv();
      setData(d);
      const f: Record<string, string> = {};
      for (const kk of d.keys) {
        f[kk] = d.values[kk] ?? "";
      }
      setForm(f);
      setInitial({ ...f });
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex));
    } finally {
      setSaving(false);
    }
  }

  if (err && !data) {
    return (
      <div className="page">
        <p className="warn">{err}</p>
        <p className="lede lede--tight">Ensure you are a platform admin and the API has a managed env path (set CM_WORKDIR or CM_MANAGED_ENV on the process).</p>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="page">
        <p className="muted">Loading…</p>
      </div>
    );
  }

  return (
    <div className="page">
      <div className="page__hero page__hero--tight">
        <h1 className="page__title">Server environment</h1>
        <p className="lede lede--tight">
          Values are written to <code className="page__code">{data.path}</code>{" "}
          {data.fileExists ? "" : "(new file on save)"} and take effect after the API and worker restart. The app
          loads this file at start (after the shell environment) so you can set <strong>CM_BASE_URL</strong> to your
          public IP or host and store tokens (e.g. <strong>CM_DEV_BEARER</strong> or <strong>CM_OIDC_CLIENT_SECRET</strong>).
        </p>
        <p className="lede lede--tight">
          {data.restartRequired}
        </p>
      </div>
      {ok && <div className="form-alert form-alert--ok">{ok}</div>}
      {err && data && <div className="form-alert form-alert--err">{err}</div>}
      <form className="vz-form vz-card" onSubmit={onAdminEnvSubmit} style={{ maxWidth: "36rem" }}>
        <h2 className="vz-form__section">Configuration keys</h2>
        {data.keys.map((k) => {
          const secret = SERVER_SECRET_KEYS.has(k);
          return (
            <label key={k} className="field">
              <span className="field__label">{ENV_LABELS[k] || k}</span>
              <span className="field__key">{k}</span>
              <input
                className="field__input"
                name={k}
                type={secret ? "password" : "text"}
                autoComplete="off"
                value={form[k] ?? ""}
                onChange={(e) => setForm((f) => ({ ...f, [k]: e.target.value }))}
                placeholder={secret && data.secretSet?.[k] ? "(set — type a new value to replace)" : ""}
              />
            </label>
          );
        })}
        <div className="form-actions">
          <button className="btn btn--primary" type="submit" disabled={saving}>
            {saving ? "Saving…" : "Save to managed.env"}
          </button>
        </div>
      </form>
    </div>
  );
}

function SettingsPage({ me }: { me: Me }) {
  const [loading, setLoading] = useState(true);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [saveErr, setSaveErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveOk, setSaveOk] = useState(false);
  const [configured, setConfigured] = useState(false);
  const [baseUrl, setBaseUrl] = useState("");
  const [pveUser, setPveUser] = useState("");
  const [tokenId, setTokenId] = useState("");
  const [secret, setSecret] = useState("");
  const [resourcePool, setResourcePool] = useState("");
  const [verifyTls, setVerifyTls] = useState(false);

  const orgId = getOrg() || (me.orgIdHeader as string) || "";

  useEffect(() => {
    if (!orgId) {
      setLoading(false);
      return;
    }
    if (!canConfigurePve(me)) {
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoadErr(null);
    getPveConnection()
      .then((d) => {
        if (cancelled) return;
        if (d.configured) {
          setConfigured(true);
          setBaseUrl(d.baseUrl);
          setPveUser(d.pveUser);
          setTokenId(d.tokenId);
          setResourcePool(d.resourcePool);
          setVerifyTls(d.verifyTls);
        } else {
          setConfigured(false);
        }
      })
      .catch((e) => {
        if (!cancelled) setLoadErr(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [me.user.id, orgId, me.orgRole, me.user.isPlatformAdmin]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaveErr(null);
    setSaveOk(false);
    setSaving(true);
    try {
      await postPveConnection({
        baseUrl,
        pveUser,
        tokenId,
        secret,
        resourcePool,
        verifyTls,
      });
      setSaveOk(true);
      setSecret("");
      setConfigured(true);
    } catch (err) {
      setSaveErr(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  if (!orgId) {
    return (
      <div className="page">
        <div className="page__hero page__hero--tight">
          <h1 className="page__title">Proxmox environment</h1>
          <p className="lede lede--tight">Select an organization in the header first.</p>
        </div>
      </div>
    );
  }

  if (!canConfigurePve(me)) {
    return (
      <div className="page">
        <div className="page__hero page__hero--tight">
          <h1 className="page__title">Proxmox environment</h1>
          <p className="lede lede--tight">
            Only <strong>org administrators</strong> (or platform admins) can set the Proxmox API token for this
            organization. Ask an admin to configure it, or use an account with the org_admin role.
          </p>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="page">
        <p className="muted">Loading current settings…</p>
      </div>
    );
  }

  if (loadErr) {
    return (
      <div className="page">
        <p className="warn">{loadErr}</p>
      </div>
    );
  }

  return (
    <div className="page">
      <div className="page__hero page__hero--tight">
        <h1 className="page__title">Proxmox environment</h1>
        <p className="lede lede--tight">
          Connect this organization to your cluster using a Proxmox <strong>API token</strong> (recommended) or
          user credentials issued as a token. Values are stored encrypted; the token secret is never shown back. See
          the official <a href="https://pve.proxmox.com/wiki/Proxmox_VE_API" target="_blank" rel="noreferrer">
            Proxmox VE API
          </a>{" "}
          overview (URL, <code className="page__code">PVEAPIToken=…</code>) and the{" "}
          <a href="https://pve.proxmox.com/pve-docs/api-viewer/" target="_blank" rel="noreferrer">
            API viewer
          </a>{" "}
          for PVE2 JSON paths (e.g. <code className="page__code">/api2/json/version</code>, nodes, qemu, lxc, vncproxy).
        </p>
      </div>

      {saveOk && <div className="form-alert form-alert--ok">Saved. You can use Compute to verify the connection.</div>}
      {saveErr && <div className="form-alert form-alert--err">{saveErr}</div>}

      <form className="vz-form vz-card" onSubmit={onSubmit}>
        <h2 className="vz-form__section">Connection</h2>
        <label className="field">
          <span className="field__label">PVE base URL</span>
          <input
            className="field__input"
            name="baseUrl"
            value={baseUrl}
            onChange={(e) => setBaseUrl(e.target.value)}
            placeholder="https://pve.example.com:8006"
            required
            autoComplete="url"
          />
          <span className="field__hint">
            HTTPS to one node (port 8006) — the <strong>origin</strong> only, e.g. <code className="page__code">
              https://10.0.0.1:8006
            </code>{" "}
            (if you paste <code className="page__code">…/api2/json</code> from the docs, the server normalizes it). The
            host/IP must match the certificate (SANs). If you see{" "}
            <code className="page__code">certificate is valid for … not 10.0…</code>, either use the IP that is on the
            cert, fix the PVE/node cert, or turn off <strong>Verify TLS</strong> below for testing only.
          </span>
        </label>
        <label className="field">
          <span className="field__label">PVE user id</span>
          <input
            className="field__input"
            name="pveUser"
            value={pveUser}
            onChange={(e) => setPveUser(e.target.value)}
            placeholder="root@pam"
            required
            autoComplete="username"
          />
          <span className="field__hint">
            Realm user only — e.g. <code className="page__code">root@pam</code>. Do <strong>not</strong> add a trailing{" "}
            <code className="page__code">!</code> (that belongs between user and token id, not here).
          </span>
        </label>
        <label className="field">
          <span className="field__label">Token id</span>
          <input
            className="field__input"
            name="tokenId"
            value={tokenId}
            onChange={(e) => setTokenId(e.target.value)}
            placeholder="test"
            required
            autoComplete="off"
          />
          <span className="field__hint">
            Only the short id after the <code className="page__code">!</code> in the Proxmox user interface (e.g.{" "}
            <code className="page__code">test</code> for <code className="page__code">root@pam!test</code>
            ) — not the full <code className="page__code">user!name</code> and not the secret. Pasting the full id
            will be auto-fixed on save.
          </span>
        </label>
        <label className="field">
          <span className="field__label">API token secret</span>
          <input
            className="field__input"
            name="secret"
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            placeholder={configured ? "Leave blank to keep the current secret" : "Token secret (uuid or value from PVE)"}
            autoComplete="new-password"
          />
        </label>
        <h2 className="vz-form__section">Options</h2>
        <label className="field">
          <span className="field__label">Resource pool (optional)</span>
          <input
            className="field__input"
            name="resourcePool"
            value={resourcePool}
            onChange={(e) => setResourcePool(e.target.value)}
            placeholder="e.g. pool/myorg"
            autoComplete="off"
          />
        </label>
        <label className="field field--row">
          <input type="checkbox" checked={verifyTls} onChange={(e) => setVerifyTls(e.target.checked)} />
          <span>
            <strong>Strict TLS / verify certificate</strong> — turn <strong>on</strong> only if the PVE HTTPS cert is
            trusted (public CA or your CA) and matches this URL. Leave <strong>off</strong> (default) for self-signed,
            snake-oil, or hostname/IP mismatch vs cert SANs (typical lab clusters).
          </span>
        </label>
        <div className="form-actions">
          <button className="btn btn--primary" type="submit" disabled={saving}>
            {saving ? "Saving…" : "Save Proxmox connection"}
          </button>
        </div>
      </form>
    </div>
  );
}

function ComputePage() {
  const [d, setD] = useState<{
    vms: { node: string; vmid: number; name: string; status: string; type: string; kind: string }[];
    nodes: string[];
  } | null>(null);
  const [e, setE] = useState<string | null>(null);
  const [consoleKey, setConsoleKey] = useState<string | null>(null);
  const [consErr, setConsErr] = useState<string | null>(null);
  useEffect(() => {
    getVMs()
      .then(setD)
      .catch((x) => setE(String(x)));
  }, []);
  if (e) {
    return (
      <div className="page">
        <div className="page__hero page__hero--tight">
          <h1 className="page__title">Compute</h1>
        </div>
        <div className="form-alert form-alert--err" style={{ maxWidth: "40rem" }}>
          <p style={{ margin: "0 0 0.5rem", fontWeight: 600 }}>Could not load guests</p>
          <p style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{e}</p>
        </div>
        <p className="lede" style={{ maxWidth: "40rem" }}>
          <strong>502 / auth errors</strong> from the API usually mean the service could not reach Proxmox (network/TLS)
          or the API token was rejected.           If the message mentions <strong>decrypt</strong> or <strong>CM_ENCRYPTION_KEY</strong>, the server
          encryption key was rotated: an org admin must re-enter and save the API token in{" "}
          <Link to="/settings">Proxmox env</Link>. In Proxmox, create an API token (user e.g.{" "}
          <code className="page__code">root@pam</code>, one token <em>name</em> with no exclamation mark); put the
          full user id in <strong>PVE user id</strong> and the token <em>name</em> and <em>secret</em> in the
          matching fields. The API host must reach <code className="page__code">https://&lt;your-pve&gt;:8006</code>{" "}
          (or your proxy) for the PVE API.
        </p>
      </div>
    );
  }
  if (!d) {
    return <p className="muted">Loading instances… (configure PVE in API first)</p>;
  }
  return (
    <div className="page">
      <div className="page__hero page__hero--tight">
        <h1 className="page__title">Compute</h1>
        <p className="lede lede--tight">
          Nodes: {d.nodes.join(", ") || "—"}. <strong>Console</strong> opens the Proxmox noVNC page on the same
          <code> base URL</code> you set under Proxmox env — your browser must reach that host on port 8006 (or your
          proxy), same model as the community{" "}
          <a href="https://github.com/zzantares/ProxmoxVE/issues/17" target="_blank" rel="noreferrer">
            vncproxy + vncwebsocket
          </a>{" "}
          flow.
        </p>
      </div>
      {consErr && <p className="form-alert form-alert--err" style={{ maxWidth: "100%" }}>{consErr}</p>}
      <div className="table-wrap">
        <table className="table">
        <thead>
          <tr>
            <th>Node</th>
            <th>ID</th>
            <th>Type</th>
            <th>Name</th>
            <th>Status</th>
            <th>Console</th>
          </tr>
        </thead>
        <tbody>
          {d.vms.length === 0 && (
            <tr>
              <td colSpan={6} className="muted">
                {d.nodes.length > 0
                  ? "No virtual machines or containers on the listed node(s) — the connection is working. If you expect guests here, check that they exist on this node and that the API token has permission to list them (PVE ACLs / token scope)."
                  : "No guests (or PVE not connected)"}
              </td>
            </tr>
          )}
          {d.vms.map((v, i) => {
            const kind = (v.kind || "qemu") === "lxc" ? "lxc" : "qemu";
            const k = `${v.node}-${v.vmid}-${i}`;
            return (
            <tr key={k}>
              <td>{v.node}</td>
              <td>{v.vmid}</td>
              <td className="mono">{v.type || kind}</td>
              <td>{v.name || "—"}</td>
              <td>
                {String(v.status).toLowerCase().includes("run") ? (
                  <span className="badge-ok">{v.status}</span>
                ) : (
                  v.status
                )}
              </td>
              <td>
                <button
                  type="button"
                  className="btn btn--primary btn--sm"
                  disabled={consoleKey === k}
                  onClick={async () => {
                    setConsErr(null);
                    setConsoleKey(k);
                    try {
                      const { url } = await postPveConsole({ node: v.node, vmid: v.vmid, kind });
                      const w = window.open(url, "_blank", "noopener,noreferrer");
                      if (!w) {
                        setConsErr("Pop-up blocked. Allow this site to open a new tab for the console.");
                      }
                    } catch (err) {
                      setConsErr(err instanceof Error ? err.message : String(err));
                    } finally {
                      setConsoleKey(null);
                    }
                  }}
                >
                  Open
                </button>
              </td>
            </tr>
            );
          })}
        </tbody>
      </table>
      </div>
    </div>
  );
}

function ImagesPage() {
  const [d, setD] = useState<{ templates: { name: string; pveNode: string; templateVmid: number }[] } | null>(null);
  const [e, setE] = useState<string | null>(null);
  useEffect(() => {
    getTemplates()
      .then(setD)
      .catch((x) => setE(String(x)));
  }, []);
  if (e) {
    return <p className="warn">{e}</p>;
  }
  if (!d) {
    return <p>Loading…</p>;
  }
  return (
    <div className="page">
      <div className="page__hero page__hero--tight">
        <h1 className="page__title">Template catalog</h1>
        <p className="lede lede--tight">Images approved for this organization.</p>
      </div>
      <div className="table-wrap">
        <table className="table">
        <thead>
          <tr><th>Name</th><th>Node</th><th>Template VMID</th></tr>
        </thead>
        <tbody>
          {d.templates.length === 0 && <tr><td colSpan={3} className="muted">No templates yet</td></tr>}
          {d.templates.map((t) => (
            <tr key={t.name}><td>{t.name}</td><td>{t.pveNode}</td><td>{t.templateVmid}</td></tr>
          ))}
        </tbody>
      </table>
      </div>
    </div>
  );
}

function ActivityPage() {
  const [d, setD] = useState<{ items: { id: string; action: string; at: unknown }[] } | null>(null);
  const [e, setE] = useState<string | null>(null);
  useEffect(() => {
    getAudit()
      .then(setD)
      .catch((x) => setE(String(x)));
  }, []);
  if (e) {
    return <p className="warn">{e}</p>;
  }
  if (!d) {
    return <p>Loading…</p>;
  }
  return (
    <div className="page">
      <div className="page__hero page__hero--tight">
        <h1 className="page__title">Activity</h1>
        <p className="lede lede--tight">Recent actions recorded for the selected org.</p>
      </div>
      <div className="table-wrap">
        <table className="table">
        <thead>
          <tr><th>ID</th><th>Action</th><th>Time</th></tr>
        </thead>
        <tbody>
          {d.items.map((a) => (
            <tr key={String(a.id)}><td>{a.id}</td><td>{a.action}</td><td>{String(a.at)}</td></tr>
          ))}
        </tbody>
      </table>
      </div>
    </div>
  );
}

function TerraformPage() {
  const [d, setD] = useState<{ workspaces: { id: string; name: string; tfVersion: string }[] } | null>(null);
  const [e, setE] = useState<string | null>(null);
  useEffect(() => {
    getTFWorkspaces()
      .then(setD)
      .catch((x) => setE(String(x)));
  }, []);
  if (e) {
    return <p className="warn">{e}</p>;
  }
  if (!d) {
    return <p>Loading…</p>;
  }
  return (
    <div className="page">
      <div className="page__hero page__hero--tight">
        <h1 className="page__title">Terraform</h1>
        <p className="lede lede--tight">
          Upload config bundles and enqueue plan/apply; the worker runs with a pinned Terraform binary.
        </p>
      </div>
      <div className="table-wrap">
        <table className="table">
        <thead>
          <tr><th>Workspace</th><th>Terraform</th><th>ID</th></tr>
        </thead>
        <tbody>
          {d.workspaces.length === 0 && <tr><td colSpan={3} className="muted">No workspaces</td></tr>}
          {d.workspaces.map((w) => (
            <tr key={w.id}><td>{w.name}</td><td>{w.tfVersion}</td><td className="mono">{w.id}</td></tr>
          ))}
        </tbody>
      </table>
      </div>
    </div>
  );
}
