const ORG = "X-Cloudmanager-Org";

function headers(orgId: string, extra: Record<string, string> = {}): HeadersInit {
  const h: Record<string, string> = { "Content-Type": "application/json", ...extra };
  if (orgId) h[ORG] = orgId;
  return h;
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
  if (!r.ok) throw new Error("vms " + r.status);
  return r.json() as Promise<{ vms: unknown[]; nodes: string[] }>;
}

export async function getTemplates() {
  const r = await fetch("/api/v1/templates", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error("tmpl " + r.status);
  return r.json() as Promise<{ templates: unknown[] }>;
}

export async function getAudit() {
  const r = await fetch("/api/v1/audit", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error("audit " + r.status);
  return r.json() as Promise<{ items: unknown[] }>;
}

export async function getTFWorkspaces() {
  const r = await fetch("/api/v1/tf/workspaces", { credentials: "include", headers: headers(getOrg()) });
  if (!r.ok) throw new Error("tf " + r.status);
  return r.json() as Promise<{ workspaces: { id: string; name: string; providerVersion: string; tfVersion: string }[] }>;
}
