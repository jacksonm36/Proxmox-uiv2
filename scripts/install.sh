#!/usr/bin/env bash
# One-shot install: check tools, optional Postgres (scripts/init-db.sql), generate .env, build API/worker, build web.
# Usage: ./scripts/install.sh [--skip-db]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SKIP_DB=0
for a in "$@"; do
  case "$a" in
    --skip-db) SKIP_DB=1 ;;
  esac
done

die() { echo "install: $*" >&2; exit 1; }
log() { echo "==> $*"; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing command: $1"; }

log "cloudmanager — install in $ROOT"

need_cmd go
need_cmd npm
need_cmd node
need_cmd openssl
need_cmd awk

# Optional: warn if old Go
GOV="$(go env GOVERSION 2>/dev/null || echo 'unknown')"
log "using $GOV"

DATA_DIR="${ROOT}/data"
WORKDIR="${DATA_DIR}/work"
WEB_DIST="${ROOT}/apps/web/dist"
OUT="${ROOT}/dist"
mkdir -p "$WORKDIR" "$OUT" "$DATA_DIR"
chmod 700 "$DATA_DIR" 2>/dev/null || true

SESS_B64="$(openssl rand -base64 48 | tr -d '\n\r' | tr -d ' ')"
ENC_HEX="$(openssl rand -hex 32)"

if [[ -f "$ROOT/.env" ]]; then
  BAK="$ROOT/.env.bak.$(date +%Y%m%d%H%M%S)"
  log "backup .env to $BAK"
  cp -a "$ROOT/.env" "$BAK" || true
fi

# Safe .env: awk replaces known keys (values with special chars work as single awk -v string on bash)
export AWK_SESS="$SESS_B64" AWK_ENC="$ENC_HEX" AWK_WD="$WORKDIR" AWK_WWW="$WEB_DIST"
awk '
  BEGIN {
    s = ENVIRON["AWK_SESS"]
    e = ENVIRON["AWK_ENC"]
    w = ENVIRON["AWK_WD"]
    www = ENVIRON["AWK_WWW"]
  }
  /^CM_SESSION_SECRET=/ { $0 = "CM_SESSION_SECRET=" s }
  /^CM_ENCRYPTION_KEY=/  { $0 = "CM_ENCRYPTION_KEY=" e }
  /^CM_WORKDIR=/         { $0 = "CM_WORKDIR=" w }
  /^CM_WEB_ROOT=/         { $0 = "CM_WEB_ROOT=" www }
  { print }
' "$ROOT/.env.example" > "$ROOT/.env"
unset AWK_SESS AWK_ENC AWK_WD AWK_WWW

if [[ "$SKIP_DB" -eq 0 ]]; then
  if command -v psql >/dev/null 2>&1; then
    if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
      log "PostgreSQL: applying scripts/init-db.sql as user postgres"
      if sudo -u postgres psql -v ON_ERROR_STOP=1 -f "$ROOT/scripts/init-db.sql" 2>/dev/null; then
        log "role cloudmanager ready (dev password: cloudmanager)"
      else
        log "init-db.sql (role) did not complete; you may need: sudo -u postgres psql -f $ROOT/scripts/init-db.sql"
      fi
      if sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='cloudmanager'" 2>/dev/null | grep -q 1; then
        log "database 'cloudmanager' already exists"
      else
        if sudo -u postgres createdb -O cloudmanager cloudmanager 2>/dev/null; then
          log "created database 'cloudmanager'"
        else
          log "createdb failed (permissions?). Create DB manually: sudo -u postgres createdb -O cloudmanager cloudmanager"
        fi
      fi
    else
      log "no non-interactive sudo: skip auto DB. Run: sudo -u postgres psql -f $ROOT/scripts/init-db.sql"
    fi
  else
    log "psql not in PATH: create DB per docs/INSTALL.md; CM_DATABASE_URL is in .env"
  fi
else
  log "skipped database bootstrap (--skip-db)"
fi

log "Go: mod tidy + build → $OUT/"
go mod tidy
go build -trimpath -ldflags "-s -w" -o "$OUT/cloudmanager-api" "./cmd/api"
go build -trimpath -ldflags "-s -w" -o "$OUT/cloudmanager-worker" "./cmd/worker"

log "Web: npm install + build"
( cd "$ROOT/apps/web" && npm install && npm run build )

cat <<EOF

----------------------------------------------------------------
Install complete.

  $OUT/cloudmanager-api
  $OUT/cloudmanager-worker
  Static UI: $WEB_DIST
  .env:      $ROOT/.env
  workdir:   $WORKDIR

From repo root (bash):

  set -a; source .env; set +a
  $OUT/cloudmanager-api
  (second terminal) $OUT/cloudmanager-worker
  (third) cd apps/web && npm run dev  # or serve dist with any static file server
----------------------------------------------------------------
EOF
