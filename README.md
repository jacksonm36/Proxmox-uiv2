# Cloudmanager

Horizon-style control plane for [Proxmox VE](https://proxmox.com/): org-scoped self-service, template catalog, audit, and isolated Terraform runs.

## Quick start (development)

### One-command install (builds the whole stack)

- **Linux / macOS / Git Bash:** `./install` or `make install` (runs [`scripts/install.sh`](scripts/install.sh))
- **Windows (PowerShell or `install.cmd`):** `.\install.cmd`  or  `powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1`

This will: check **Go / Node / openssl**; back up and regenerate **`.env`** (session + encryption keys, `data/work` workdir, `apps/web/dist` as `CM_WEB_ROOT`); on Linux, try **`sudo -u postgres`** to apply [`scripts/init-db.sql`](scripts/init-db.sql); run **`go mod tidy`** and build **`dist/cloudmanager(-worker)`**; run **`npm install` + `npm run build`** in `apps/web`.

Use `scripts/install.sh --skip-db` (or `install.ps1 -SkipDb`) if PostgreSQL is not ready yet.

Then load `.env` and start `dist/cloudmanager-api`, `dist/cloudmanager-worker`, and `cd apps/web && npm run dev` (see the script footer).

If you do not use the install script, continue below.

From the repository root, you can also run `go mod tidy` once to generate `go.sum` (requires Go 1.22+).

### 1. PostgreSQL (native install)

Use a **locally installed** PostgreSQL 15+ service (not Docker) for development. Detailed steps: [docs/INSTALL.md](docs/INSTALL.md#native-postgresql-linux-and-windows).

**Short version:** create a database and a role, then point `CM_DATABASE_URL` at it.

**Linux (Debian/Ubuntu example) — or use `./install` which runs this for you if sudo is non-interactive:**

```bash
sudo apt install postgresql
sudo -u postgres psql -f scripts/init-db.sql
sudo -u postgres createdb -O cloudmanager cloudmanager
# Optional: allow local password auth — edit /etc/postgresql/*/main/pg_hba.conf, then:
sudo systemctl restart postgresql
```

**Windows:** install from [EnterpriseDB](https://www.postgresql.org/download/windows/) or `winget install PostgreSQL.PostgreSQL`, then use **pgAdmin** or `psql` to create the `cloudmanager` user and `cloudmanager` database (same idea as above).

### 2. App configuration

```bash
cp .env.example .env
# Edit .env: CM_SESSION_SECRET, CM_ENCRYPTION_KEY, and CM_DATABASE_URL to match your Postgres user/db.
```

`CM_REDIS_ADDR` in `.env` is **optional** (this codebase uses a Postgres job queue; Redis is not required to run the API and worker). You can remove or leave it unset.

### 3. Run the services

**API (repo root):**

```bash
go run ./cmd/api
```

**Worker (separate terminal):**

```bash
go run ./cmd/worker
```

**Web UI (separate terminal):**

```bash
cd apps/web && npm install && npm run dev
```

Open `http://localhost:5173` — the Vite dev server proxies `/api` to the API (default `http://localhost:8080`).

### Optional: Docker only for dependencies

If you prefer containers for Postgres/Redis only:

```bash
docker compose up -d
```

The defaults in `docker-compose.yml` still match the sample `CM_DATABASE_URL` in `.env.example`.

For production, use the Debian/RPM templates under [packaging/](packaging) and unit files in [deploy/systemd/](deploy/systemd). See [docs/INSTALL.md](docs/INSTALL.md) and [docs/MSP_HARDENING.md](docs/MSP_HARDENING.md).

**Note:** Full browser OIDC redirect (`/api/v1/auth/login` / `callback`) can be added by wiring `internal/auth/oidc.go` to exchange the code and issue the same JWT cookie as `GET /api/v1/auth/dev`. Until then, use dev bootstrap + dev login for local testing, or API keys with `Authorization: Bearer cm_…` and `X-Cloudmanager-Org`.

## License

Proprietary / choose your license.
