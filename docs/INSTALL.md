# Installing Cloudmanager (Linux packages + systemd)

## One-shot dev install (from a git clone)

1. **Linux / macOS / Git Bash (from the repo root):** `./install` or `make install`  
2. **Windows:** `install.cmd` or `powershell -ExecutionPolicy Bypass -File scripts\install.ps1`

The script runs [`scripts/init-db.sql`](scripts/init-db.sql) and `createdb` on Linux if **non-interactive** `sudo -u postgres` is available. On Windows it prints the path to the SQL file; run it in **pgAdmin** or `psql`, then `CREATE DATABASE` / `createdb` as in [Native PostgreSQL](#native-postgresql-linux-and-windows). After a successful run you get **`dist/cloudmanager-api`**, **`dist/cloudmanager-worker`**, and a built **`apps/web/dist`**.

## Requirements

- PostgreSQL 15+ (native install recommended; see below)
- Go 1.23+ (to build from source) or prebuilt binaries from your pipeline
- Node 20+ (only to build the web UI)
- `terraform` CLI on the worker host (version pinned per workspace in the database; install a matching default in `/usr/bin/terraform` or on `PATH`)

## Native PostgreSQL (Linux and Windows)

The application expects a normal PostgreSQL server with a **database** and a **user** the API can use. Migrations run automatically when the API starts (or you can use `goose` on `internal/migrate/migrations` if you prefer).

### Linux (Debian / Ubuntu)

```bash
sudo apt update
sudo apt install -y postgresql
```

Create role and database (replace the password in production):

```bash
sudo -u postgres psql -v ON_ERROR_STOP=1 -f scripts/init-db.sql
sudo -u postgres createdb -O cloudmanager cloudmanager
```

(The `./install` script runs the same when non-interactive `sudo -u postgres` is available.)

If you use a Unix socket and `peer` auth for `localhost`, you may need a line in `pg_hba.conf` to allow the app user to connect with a password, for example (adjust path; often `/etc/postgresql/<version>/main/pg_hba.conf`):

```
# IPv4 local connections — password for app user
host    all    all    127.0.0.1/32    scram-sha-256
```

Then:

```bash
sudo systemctl restart postgresql
```

`CM_DATABASE_URL` example:

`postgres://cloudmanager:cloudmanager@127.0.0.1:5432/cloudmanager?sslmode=disable`

### RHEL / Fedora / Rocky (dnf)

```bash
sudo dnf install -y postgresql-server postgresql-contrib
sudo postgresql-setup --initdb  # on some versions
sudo systemctl enable --now postgresql
sudo -u postgres createuser -P cloudmanager   # set password
sudo -u postgres createdb -O cloudmanager cloudmanager
```

### Windows

1. Install PostgreSQL 15+ using the [official Windows installer](https://www.postgresql.org/download/windows/) or, for example, `winget install PostgreSQL.PostgreSQL` (package name may vary).
2. Open **pgAdmin** or run **SQL Shell (psql)** and connect as the `postgres` superuser.
3. Create a login role `cloudmanager` with a password, then create a database `cloudmanager` owned by that role (same as the Linux `CREATE USER` / `CREATE DATABASE` steps above).
4. In `.env`, set `CM_DATABASE_URL=postgres://cloudmanager:YOUR_PASSWORD@127.0.0.1:5432/cloudmanager?sslmode=disable` (default port is 5432 unless you changed it during install).
5. If the service listens only on `::1`, use `host=127.0.0.1` in the URL as shown, or the IPv6 host your server is bound to.

No Docker is required; `docker compose` in the repo is **optional** for teams that still want a containerized Postgres for parity with older docs.

## Configuration

1. Create system user `cloudmanager` and group `cloudmanager-terraform` (worker runs Terraform as a separate group; adjust `deploy/systemd` to match your policy).

2. Copy [deploy/systemd/cloudmanager-api.service](../deploy/systemd/cloudmanager-api.service) and [cloudmanager-worker.service](../deploy/systemd/cloudmanager-worker.service) to `/etc/systemd/system/` and [cloudmanager.tmpfiles.conf](../deploy/systemd/cloudmanager.tmpfiles.conf) to `/usr/lib/tmpfiles.d/`.

3. Create `/etc/cloudmanager/environment` (or `EnvironmentFile` path you prefer) with at least:

   - `CM_DATABASE_URL=postgres://cloudmanager:...@localhost:5432/cloudmanager?sslmode=disable`
   - `CM_SESSION_SECRET=` (32+ random bytes)
   - `CM_ENCRYPTION_KEY=` (64 hex chars = 32-byte AES key)
   - `CM_BASE_URL=https://cloudmanager.example.com`
   - `CM_CORS_ORIGINS=https://cloudmanager.example.com`
   - `CM_WORKDIR=/var/lib/cloudmanager`
   - `CM_OIDC_ISSUER`, `CM_OIDC_CLIENT_ID`, `CM_OIDC_REDIRECT_URL` (omit or set `CM_DISABLE_OIDC=1` only for lab; production must use OIDC)

4. Build the API and worker:

   ```bash
   go build -o cloudmanager-api ./cmd/api
   go build -o cloudmanager-worker ./cmd/worker
   sudo install -m755 cloudmanager-api cloudmanager-worker /usr/bin/
   ```

5. Build the web UI and install static files:

   ```bash
   cd apps/web && npm install && npm run build
   sudo mkdir -p /usr/share/cloudmanager/web
   sudo cp -r apps/web/dist/* /usr/share/cloudmanager/web/
   ```

6. Set `CM_WEB_ROOT=/usr/share/cloudmanager/web` in the environment file for the API.

7. Run SQL migrations (on first start the API also runs embedded migrations, or use goose against `internal/migrate/migrations`).

8. `sudo systemd-tmpfiles --create` then `systemctl enable --now cloudmanager-api cloudmanager-worker`.

## Proxmox per org (MSP)

Each customer org should use a **dedicated** Proxmox API token and ACLs on a **resource pool**; configure the connection via `POST /api/v1/pve/connection` with the UI or API. See [MSP_HARDENING.md](MSP_HARDENING.md).

## Terraform

The worker runs `terraform` in a temporary directory and extracts `bundleB64` uploads. Pin versions in the workspace; install a compatible Terraform binary on the worker host. For stronger isolation, run the worker in a container or a dedicated host with `ProtectSystem=strict` and a firewall allowing only the Proxmox API endpoint.

## Debian / RPM

Packaging examples live under [packaging/](../packaging). Adapt `debian/` or the `.spec` to your build pipeline; they expect `go build` in `%build` or `debian/rules` and install the same FHS layout as above.
