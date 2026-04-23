const ORG = "X-Cloudmanager-Org";

function headers(orgId: string, extra: Record<string, string> = {}): HeadersInit {
  const h: Record<string, string> = { "Content-Type": "application/json", ...extra };
  if (orgId) h[ORG] = orgId;
  return h;
}

/** Parse { "error": "..." } from failed API JSON responses. */
export async function readApiError(r: Response, what: string): Promise<string> {
  const j = (await r.json().catch(() => ({}))) as { error?: string };
  if (j.error) return j.error;
  return `${what} failed (${r.status})`;
}

export async function fetchMe(creds: "include" | "omit" = "include") {
  const r = await fetch("/api/v1/auth/me", { credentials: creds, headers: headers(getOrg() || "") });
  if (!r.ok) throw new Error("me " + r.status);
  return r.json() as Promise<{
    user: { id: string; email: string; isPlatformAdmin: boolean };
    orgs: { org: { id: string; name: string; slug: string }; role: string; orgId: string }[];
    orgIdHeader: string;
    orgRole: string;
  }>;
}

export type PveConnectionGet =
  | { configured: false }
  | {
      configured: true;
      baseUrl: string;
      pveUser: string;
      tokenId: string;
      resourcePool: string;
      verifyTls: boolean;
    };

/** Proxmox API connection (per org; requires org admin or platform; GET omits token secret) */
export async function getPveConnection(): Promise<PveConnectionGet> {
  const r = await fetch("/api/v1/pve/connection", { credentials: "include", headers: headers(getOrg()) });
  if (r.status === 403) throw new Error("forbidden");
  if (!r.ok) throw new Error("pve " + r.status);
  return r.json() as Promise<PveConnectionGet>;
}

export async function postPveConnection(body: {
  baseUrl: string;
  pveUser: string;
  tokenId: string;
  /** Omit or empty to keep existing secret (updates only) */
  secret: string;
  resourcePool: string;
  verifyTls: boolean;
}): Promise<void> {
  const r = await fetch("/api/v1/pve/connection", {
    method: "POST",
    credentials: "include",
    headers: headers(getOrg()),
    body: JSON.stringify({
      baseUrl: body.baseUrl,
      pveUser: body.pveUser,
      tokenId: body.tokenId,
      secret: body.secret,
      resourcePool: body.resourcePool,
      verifyTls: body.verifyTls,
    }),
  });
  if (r.status === 403) throw new Error("forbidden");
  if (!r.ok) {
    const j = (await r.json().catch(() => ({}))) as { error?: string };
    throw new Error(j.error || "save " + r.status);
  }
}

let cachedOrg: string | null = null;
export function setOrg(id: string) {
  cachedOrg = id;
  sessionStorage.setItem("cm_org", id);
}
export function getOrg(): string {
  if (cachedOrg) return cachedOrg;
  const s = sessionStorage.getItem("cm_org");
  if (s) {
    cachedOrg = s;
    return s;
  }
  return "";
}

export function initOrgFromMe(data: { orgs: { org: { id: string } }[] }) {
  if (!getOrg() && data.orgs.length > 0) {
    setOrg(data.orgs[0].org.id);
  }
}

export async function getVMs() {
  const r = await fetch("/api/v1/pve/vms", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error(await readApiError(r, "VM list"));
  return r.json() as Promise<{
    vms: { node: string; vmid: number; name: string; status: string; type: string; kind: string }[];
    nodes: string[];
  }>;
}

/** Open a noVNC session URL on the PVE node (vncproxy + ticket). Browser must reach the cluster URL. */
export type AdminEnvGet = {
  path: string;
  fileExists: boolean;
  values: Record<string, string>;
  secretSet: Record<string, boolean>;
  keys: string[];
  restartRequired: string;
};

/** Platform admin: read managed.env (secrets not returned; database URL masked). */
export async function getAdminEnv(): Promise<AdminEnvGet> {
  const r = await fetch("/api/v1/admin/env", { credentials: "include", headers: headers("") });
  if (r.status === 403) throw new Error("forbidden");
  if (!r.ok) throw new Error(await readApiError(r, "Admin env"));
  return r.json() as Promise<AdminEnvGet>;
}

/** Platform admin: patch keys (omit a key to leave unchanged; empty string removes from file). */
export async function patchAdminEnv(
  patch: Record<string, string>
): Promise<{ ok: boolean; path: string; notice: string; relogin?: string }> {
  const r = await fetch("/api/v1/admin/env", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", ...headers("") },
    body: JSON.stringify(patch),
  });
  if (r.status === 403) throw new Error("forbidden");
  if (!r.ok) {
    const j = (await r.json().catch(() => ({}))) as { error?: string };
    throw new Error(j.error || "save " + r.status);
  }
  return r.json() as Promise<{ ok: boolean; path: string; notice: string; relogin?: string }>;
}

export async function postPveConsole(body: { node: string; vmid: number; kind: "qemu" | "lxc" }): Promise<{ url: string; port: number }> {
  const r = await fetch("/api/v1/pve/console", {
    method: "POST",
    credentials: "include",
    headers: headers(getOrg()),
    body: JSON.stringify(body),
  });
  if (!r.ok) {
    const j = (await r.json().catch(() => ({}))) as { error?: string };
    throw new Error(j.error || "console " + r.status);
  }
  return r.json() as Promise<{ url: string; port: number }>;
}

export async function getTemplates() {
  const r = await fetch("/api/v1/templates", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error(await readApiError(r, "Templates"));
  return r.json() as Promise<{ templates: unknown[] }>;
}

export async function getAudit() {
  const r = await fetch("/api/v1/audit", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error(await readApiError(r, "Audit"));
  return r.json() as Promise<{ items: unknown[] }>;
}

export async function getTFWorkspaces() {
  const r = await fetch("/api/v1/tf/workspaces", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error(await readApiError(r, "Terraform"));
  return r.json() as Promise<{ workspaces: { id: string; name: string; providerVersion: string; tfVersion: string }[] }>;
}
