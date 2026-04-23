-- +goose Up
-- +goose NO TRANSACTION
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Platform identity (from OIDC `sub` or dev bootstrap)
CREATE TABLE users (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  oidc_sub   TEXT UNIQUE,
  email      TEXT NOT NULL,
  name       TEXT,
  is_platform_admin BOOLEAN NOT NULL DEFAULT FALSE,
  break_glass_until   TIMESTAMPTZ, -- super-admin time-boxed
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orgs (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL UNIQUE,
  max_vms    INT NOT NULL DEFAULT 50,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE org_memberships (
  org_id  UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role    TEXT NOT NULL CHECK (role IN ('org_admin', 'org_member')),
  PRIMARY KEY (org_id, user_id)
);
CREATE INDEX org_memberships_user_id_idx ON org_memberships(user_id);

-- Encrypted PVE connection per org (MSP: dedicated API token scoped to a pool in Proxmox ACLs)
CREATE TABLE org_pve_secrets (
  org_id   UUID PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
  base_url TEXT NOT NULL,   -- e.g. https://pve.local:8006
  pve_user TEXT NOT NULL,   -- user@realm
  -- Key ID + secret id from Proxmox token; full secret is encrypted (AES-256-GCM) in app layer
  token_id  TEXT,
  enc_token_secret BYTEA,   -- ciphertext + nonce
  resource_pool  TEXT,      -- e.g. /pool/customerA
  verify_tls       BOOLEAN NOT NULL DEFAULT TRUE,
  last_ok_at        TIMESTAMPTZ,
  last_error         TEXT,
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Service API keys (hashed) for automation per org
CREATE TABLE api_keys (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  key_prefix CHAR(8) NOT NULL, -- first bytes for display
  key_hash   BYTEA NOT NULL,  -- argon2 or bcrypt hash of secret
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ
);
CREATE INDEX api_keys_org_id_idx ON api_keys(org_id);
CREATE INDEX api_keys_prefix_idx ON api_keys(key_prefix);

-- Flavors: T-shirt sizes
CREATE TABLE flavors (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  cpu_cores  INT NOT NULL CHECK (cpu_cores > 0),
  memory_mb  INT NOT NULL CHECK (memory_mb > 0),
  disk_gb    INT NOT NULL DEFAULT 0,
  network_bridge TEXT DEFAULT 'vmbr0',
  UNIQUE (org_id, name)
);

-- Template catalog (wraps a PVE template VM on a node)
CREATE TABLE template_catalog (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  description TEXT,
  pve_node    TEXT NOT NULL,
  template_vmid INT NOT NULL, -- 9000-style template id
  default_cloudinit TEXT, -- optional #cloud-config yaml fragment
  UNIQUE (org_id, name)
);

CREATE TABLE projects (
  id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name   TEXT NOT NULL,
  slug   TEXT NOT NULL,
  UNIQUE (org_id, slug)
);

-- Append-style audit
CREATE TABLE audit_log (
  id             BIGSERIAL PRIMARY KEY,
  org_id         UUID REFERENCES orgs(id) ON DELETE SET NULL,
  actor_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
  action         TEXT NOT NULL,
  target_type    TEXT,
  target_id      TEXT,
  meta           JSONB,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_org_created_idx ON audit_log(org_id, created_at DESC);

-- Job queue: durable TF / sync jobs (post-MVP: Redis Asynq)
CREATE TABLE work_jobs (
  id         BIGSERIAL PRIMARY KEY,
  org_id     UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL,
  payload    JSONB NOT NULL,
  status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed')),
  attempts   INT NOT NULL DEFAULT 0,
  run_after  TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at  TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  error_text  TEXT
);
CREATE INDEX work_jobs_fetch_idx ON work_jobs(status, run_after) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS work_jobs;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS template_catalog;
DROP TABLE IF EXISTS flavors;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS org_pve_secrets;
DROP TABLE IF EXISTS org_memberships;
DROP TABLE IF EXISTS orgs;
DROP TABLE IF EXISTS users;
