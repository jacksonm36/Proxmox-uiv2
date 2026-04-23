import { useEffect, useState } from "react";
import { Link, NavLink, Route, Routes } from "react-router-dom";
import {
  fetchMe,
  getVMs,
  getTemplates,
  getAudit,
  getTFWorkspaces,
  getOrg,
  setOrg,
  initOrgFromMe,
} from "./api";

type Me = Awaited<ReturnType<typeof fetchMe>>;

export function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [err, setErr] = useState<string | null>(null);
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

  if (err && !me) {
    return (
      <div className="shell">
        <p className="warn">Not signed in. In development, <a href="/api/v1/auth/dev">obtain a session</a> then refresh.</p>
      </div>
    );
  }
  if (!me) {
    return <div className="shell muted">Loading…</div>;
  }

  return (
    <div className="layout">
      <header className="topbar">
        <div className="brand">Cloudmanager</div>
        <nav>
          <NavLink to="/" end>Project</NavLink>
          <NavLink to="/compute">Compute</NavLink>
          <NavLink to="/images">Images</NavLink>
          <NavLink to="/terraform">Terraform</NavLink>
          <NavLink to="/activity">Activity</NavLink>
        </nav>
        <OrgPicker me={me} />
        <div className="user">{me.user.email}</div>
      </header>
      <main>
        <Routes>
          <Route path="/" element={<Dashboard me={me} />} />
          <Route path="/compute" element={<ComputePage />} />
          <Route path="/images" element={<ImagesPage />} />
          <Route path="/terraform" element={<TerraformPage />} />
          <Route path="/activity" element={<ActivityPage />} />
        </Routes>
      </main>
    </div>
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
    <label className="org">
      <span>Org</span>
      <select
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

function Dashboard({ me }: { me: Me }) {
  return (
    <div className="page">
      <h1>Overview</h1>
      <p className="lede">Horizon-style project console for Proxmox: self-service, templates, audit, and Terraform.</p>
      <div className="cards">
        <Link className="card" to="/compute">
          <h3>Instances</h3>
          <p>View and control VMs your token can access.</p>
        </Link>
        <Link className="card" to="/images">
          <h3>Template catalog</h3>
          <p>Org-approved templates for new workloads.</p>
        </Link>
        <Link className="card" to="/terraform">
          <h3>Terraform</h3>
          <p>Workspaces, plans, and applies (worker executes runs).</p>
        </Link>
        <Link className="card" to="/activity">
          <h3>Activity</h3>
          <p>Audit log for the selected organization.</p>
        </Link>
      </div>
    </div>
  );
}

function ComputePage() {
  const [d, setD] = useState<{ vms: { node: string; vmid: number; name: string; status: string }[]; nodes: string[] } | null>(null);
  const [e, setE] = useState<string | null>(null);
  useEffect(() => {
    getVMs()
      .then(setD)
      .catch((x) => setE(String(x)));
  }, []);
  if (e) {
    return <p className="warn">{e}</p>;
  }
  if (!d) {
    return <p className="muted">Loading instances… (configure PVE in API first)</p>;
  }
  return (
    <div className="page">
      <h1>Compute</h1>
      <p className="muted">Nodes: {d.nodes.join(", ") || "—"}</p>
      <table className="table">
        <thead>
          <tr><th>Node</th><th>ID</th><th>Name</th><th>Status</th></tr>
        </thead>
        <tbody>
          {d.vms.length === 0 && (
            <tr><td colSpan={4} className="muted">No VMs (or PVE not connected)</td></tr>
          )}
          {d.vms.map((v, i) => (
            <tr key={i}>
              <td>{v.node}</td><td>{v.vmid}</td><td>{v.name || "—"}</td><td>{v.status}</td>
            </tr>
          ))}
        </tbody>
      </table>
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
      <h1>Template catalog</h1>
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
      <h1>Activity</h1>
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
      <h1>Terraform</h1>
      <p className="lede">Upload config bundles and enqueue plan/apply; the worker process runs with pinned Terraform (see install docs).</p>
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
  );
}
