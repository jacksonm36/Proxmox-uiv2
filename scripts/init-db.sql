-- Idempotent role (dev password). Run as superuser, e.g.:
--   sudo -u postgres psql -f scripts/init-db.sql
-- CREATE DATABASE must not run inside a DO block; install.sh / docs use `createdb` for the database step.

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'cloudmanager') THEN
    CREATE ROLE cloudmanager WITH LOGIN PASSWORD 'cloudmanager';
  END IF;
END
$$;
