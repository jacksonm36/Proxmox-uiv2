-- +goose Up
-- New installs: prefer skipping strict TLS (self-signed PVE is common; orgs can turn verify on in UI).
ALTER TABLE org_pve_secrets ALTER COLUMN verify_tls SET DEFAULT false;

-- +goose Down
ALTER TABLE org_pve_secrets ALTER COLUMN verify_tls SET DEFAULT true;
