-- +goose Up
-- Row-level security: set session vars app.org_id (uuid text) and app.is_platform_admin (true/false)
ALTER TABLE orgs ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_pve_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE flavors ENABLE ROW LEVEL SECURITY;
ALTER TABLE template_catalog ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tf_workspaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE tf_config_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE tf_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tf_state_locks ENABLE ROW LEVEL SECURITY;
-- users: RLS not applied — enforce in app; avoids OIDC upsert/invite edge cases. Sensitive data lives in org_pve_secrets, etc.

-- Helper: is platform session
CREATE OR REPLACE FUNCTION app_is_platform()
RETURNS boolean
LANGUAGE sql STABLE
AS $$
  SELECT coalesce(nullif(current_setting('app.is_platform_admin', true), ''), 'false')::boolean;
$$;

CREATE OR REPLACE FUNCTION app_current_org()
RETURNS uuid
LANGUAGE sql STABLE
AS $$
  SELECT nullif(nullif(current_setting('app.org_id', true), ''), 'null')::uuid;
$$;

CREATE OR REPLACE FUNCTION app_org_role()
RETURNS text
LANGUAGE sql STABLE
AS $$
  SELECT coalesce(nullif(current_setting('app.org_role', true), ''), 'org_member');
$$;

-- orgs: members see their org; platform sees all
CREATE POLICY orgs_select ON orgs FOR SELECT
  USING (app_is_platform() OR id = app_current_org());
CREATE POLICY orgs_maintain ON orgs FOR ALL
  USING (app_is_platform())
  WITH CHECK (app_is_platform());
CREATE POLICY orgs_update_tenant ON orgs FOR UPDATE
  USING (id = app_current_org() AND app_org_role() = 'org_admin' AND NOT app_is_platform())
  WITH CHECK (id = app_current_org());

-- org_memberships
CREATE POLICY org_memberships_all ON org_memberships FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

-- org_pve_secrets: never expose cross-org; org_admin only via app (extra check in code)
CREATE POLICY org_pve_secrets_all ON org_pve_secrets FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY api_keys_all ON api_keys FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY flavors_all ON flavors FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY template_catalog_all ON template_catalog FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY projects_all ON projects FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY audit_log_select ON audit_log FOR SELECT
  USING (app_is_platform() OR org_id = app_current_org());
CREATE POLICY audit_log_insert ON audit_log FOR INSERT
  WITH CHECK (app_is_platform() OR org_id = app_current_org() OR org_id IS NULL);

CREATE POLICY work_jobs_all ON work_jobs FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY tf_workspaces_all ON tf_workspaces FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY tf_config_versions_all ON tf_config_versions FOR ALL
  USING (app_is_platform() OR EXISTS (
    SELECT 1 FROM tf_workspaces w
    WHERE w.id = tf_config_versions.workspace_id AND w.org_id = app_current_org()
  ))
  WITH CHECK (app_is_platform() OR EXISTS (
    SELECT 1 FROM tf_workspaces w
    WHERE w.id = tf_config_versions.workspace_id AND w.org_id = app_current_org()
  ));

CREATE POLICY tf_runs_all ON tf_runs FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

CREATE POLICY tf_state_locks_all ON tf_state_locks FOR ALL
  USING (app_is_platform() OR org_id = app_current_org())
  WITH CHECK (app_is_platform() OR org_id = app_current_org());

-- +goose Down
DROP POLICY IF EXISTS orgs_update_tenant ON orgs;
DROP POLICY IF EXISTS orgs_maintain ON orgs;
DROP POLICY IF EXISTS orgs_select ON orgs;
DROP POLICY IF EXISTS org_memberships_all ON org_memberships;
DROP POLICY IF EXISTS org_pve_secrets_all ON org_pve_secrets;
DROP POLICY IF EXISTS api_keys_all ON api_keys;
DROP POLICY IF EXISTS flavors_all ON flavors;
DROP POLICY IF EXISTS template_catalog_all ON template_catalog;
DROP POLICY IF EXISTS projects_all ON projects;
DROP POLICY IF EXISTS audit_log_select ON audit_log;
DROP POLICY IF EXISTS audit_log_insert ON audit_log;
DROP POLICY IF EXISTS work_jobs_all ON work_jobs;
DROP POLICY IF EXISTS tf_workspaces_all ON tf_workspaces;
DROP POLICY IF EXISTS tf_config_versions_all ON tf_config_versions;
DROP POLICY IF EXISTS tf_runs_all ON tf_runs;
DROP POLICY IF EXISTS tf_state_locks_all ON tf_state_locks;
DROP FUNCTION IF EXISTS app_org_role;
DROP FUNCTION IF EXISTS app_current_org;
DROP FUNCTION IF EXISTS app_is_platform;
