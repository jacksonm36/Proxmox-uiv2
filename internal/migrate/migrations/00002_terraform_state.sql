-- +goose Up
CREATE TABLE tf_workspaces (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  description TEXT,
  provider_version TEXT NOT NULL DEFAULT 'bpg/proxmox',
  tf_version  TEXT NOT NULL DEFAULT '1.7.0',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (org_id, name)
);

CREATE TABLE tf_config_versions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id  UUID NOT NULL REFERENCES tf_workspaces(id) ON DELETE CASCADE,
  version       INT NOT NULL,
  -- Tar.gz of .tf files stored on disk; path is relative to workdir
  bundle_path   TEXT NOT NULL,
  bundle_sha256 TEXT NOT NULL,
  enc_tf_vars   BYTEA, -- encrypted JSON map, optional
  created_by    UUID REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, version)
);

CREATE TABLE tf_runs (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id         UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  workspace_id   UUID NOT NULL REFERENCES tf_workspaces(id) ON DELETE CASCADE,
  config_version_id UUID REFERENCES tf_config_versions(id),
  op             TEXT NOT NULL CHECK (op IN ('plan', 'apply', 'destroy')),
  status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'canceled')),
  log_path       TEXT,
  log_tail       TEXT, -- last N KB for list view
  state_path     TEXT, -- per-org state file path on API/worker node
  lock_token     TEXT,
  created_by     UUID REFERENCES users(id),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at     TIMESTAMPTZ,
  finished_at    TIMESTAMPTZ,
  error_summary  TEXT
);
CREATE INDEX tf_runs_ws_idx ON tf_runs(workspace_id, created_at DESC);
CREATE INDEX tf_runs_status_idx ON tf_runs(status) WHERE status IN ('pending', 'running');

CREATE TABLE tf_state_locks (
  workspace_id UUID PRIMARY KEY REFERENCES tf_workspaces(id) ON DELETE CASCADE,
  org_id        UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  run_id        UUID NOT NULL REFERENCES tf_runs(id) ON DELETE CASCADE,
  held_until    TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS tf_state_locks;
DROP TABLE IF EXISTS tf_runs;
DROP TABLE IF EXISTS tf_config_versions;
DROP TABLE IF EXISTS tf_workspaces;
