# MSP hardening checklist

Use this with multi-tenant Proxmox clusters where each customer maps to a **dedicated** Proxmox identity and **resource pool**.

## Proxmox (source of truth for isolation)

1. **Per-org API user** (e.g. `customerA@pve`) with a **API token** limited to a **pool** (`/pool/customerA`) and required paths only (`/pool/customerA`, `sdn`, `storage` as needed — least privilege).
2. **No shared** `root@pam` or `terraform@` tokens across orgs in Cloudmanager; store tokens **encrypted** (application uses `CM_ENCRYPTION_KEY`).
3. Re-verify ACLs when adding nodes or storage; automation should not expand scope silently.
4. **Audit** in Proxmox and in Cloudmanager: review `GET /api/v1/audit` regularly; ship logs to a SIEM if required.

## Cloudmanager

1. **TLS** in front of the API (reverse proxy) and for Proxmox (`verifyTls: true` on connections).
2. **OIDC** for human users; disable dev login (`CM_DEV_BOOTSTRAP=0`) in production; remove `GET /api/v1/auth/dev` exposure with network policy or a separate build.
3. **Session secret** and **encryption key** from a vault or HSM; rotate with a documented runbook; never commit secrets.
4. **Rate limiting** is enabled globally; tune `httprate` and add reverse-proxy limits for public endpoints.
5. **Break-glass platform admin**: use `is_platform_admin` and optional `break_glass_until` (schema present; wire policy in your fork) for time-boxed super access with ticket IDs in audit.
6. **Backups**: back up PostgreSQL, `/var/lib/cloudmanager` (Terraform bundles and state paths), and your OIDC/secret material. Test restores.
7. **Terraform** execution: treat as **arbitrary code**; run worker on a locked-down host, pin provider checksums, and restrict outbound firewall to PVE and required providers only.

## Rate limits and quotas

- Per-org `max_vms` in `orgs` is a soft app-side guard; **enforce** real limits in Proxmox (pool and quota features) in addition to Cloudmanager.

When in doubt, assume **PVE ACLs** are the enforcement layer; the app adds UX, audit, and Terraform workflow.
