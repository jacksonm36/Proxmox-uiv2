package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/repo"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (s *Server) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	var out []map[string]any
	err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, description, provider_version, tf_version, created_at::text
			FROM tf_workspaces WHERE org_id = $1::uuid ORDER BY name
		`, oid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, desc, pv, tv, ct string
			if err := rows.Scan(&id, &name, &desc, &pv, &tv, &ct); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"id": id, "name": name, "description": desc, "providerVersion": pv, "tfVersion": tv, "createdAt": ct,
			})
		}
		return rows.Err()
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"workspaces": out})
}

func (s *Server) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" || (OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context())) {
		s.json(w, 403, jsonErr{"forbidden"})
		return
	}
	var b struct {
		Name, Description, TFVersion, Provider string
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.json(w, 400, jsonErr{"json"})
		return
	}
	if b.TFVersion == "" {
		b.TFVersion = "1.7.0"
	}
	if b.Provider == "" {
		b.Provider = "bpg/proxmox"
	}
	var id string
	err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO tf_workspaces (org_id, name, description, provider_version, tf_version)
			VALUES ($1::uuid, $2, $3, $4, $5)
			RETURNING id::text
		`, oid, b.Name, b.Description, b.Provider, b.TFVersion).Scan(&id)
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "tf.workspace.create", "tf_workspace", id)
	s.json(w, 200, map[string]any{"id": id})
}

// UploadConfig POST /api/v1/tf/workspaces/{id}/config
func (s *Server) UploadConfig(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	wsID := chi.URLParam(r, "id")
	var b struct {
		BundleB64 string `json:"bundleB64"`
		SHA256    string `json:"sha256"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.json(w, 400, jsonErr{"json"})
		return
	}
	raw, err := base64.StdEncoding.DecodeString(b.BundleB64)
	if err != nil {
		s.json(w, 400, jsonErr{"invalid base64"})
		return
	}
	dir := filepath.Join(s.Cfg.Workdir, "tf-bundles", oid, wsID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	var ver int
	if err := s.Pool.QueryRow(r.Context(), `SELECT coalesce(max(version),0)+1 FROM tf_config_versions WHERE workspace_id = $1::uuid`, wsID).Scan(&ver); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	fname := fmt.Sprintf("bundle-v%d.tgz", ver)
	fpath := filepath.Join(dir, fname)
	if err := os.WriteFile(fpath, raw, 0o600); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	var cid string
	err = db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO tf_config_versions (workspace_id, version, bundle_path, bundle_sha256, created_by)
			VALUES ($1::uuid, $2, $3, $4, $5::uuid)
			RETURNING id::text
		`, wsID, ver, fpath, b.SHA256, UserID(r.Context())).Scan(&cid)
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"configVersionId": cid, "version": ver, "path": fpath})
}

func (s *Server) EnqueuePlan(w http.ResponseWriter, r *http.Request) {
	s.enqueueTF(w, r, "plan")
}

func (s *Server) EnqueueApply(w http.ResponseWriter, r *http.Request) {
	s.enqueueTF(w, r, "apply")
}

func (s *Server) enqueueTF(w http.ResponseWriter, r *http.Request, op string) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	wsID := chi.URLParam(r, "id")
	if wsID == "" {
		s.json(w, 400, jsonErr{"missing workspace id"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct{ ConfigVersionID *string `json:"configVersionId"` }
	_ = json.Unmarshal(body, &req)
	uid := UserID(r.Context())
	var runID, statePath string
	statePath = filepath.Join(s.Cfg.Workdir, "tfstate", oid, wsID, "default.tfstate")
	_ = os.MkdirAll(filepath.Dir(statePath), 0o750)
	err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		var cvi any
		if req.ConfigVersionID != nil {
			cvi = *req.ConfigVersionID
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO tf_runs (org_id, workspace_id, config_version_id, op, status, state_path, created_by)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'pending', $5, $6::uuid)
			RETURNING id::text
		`, oid, wsID, cvi, op, statePath, uid).Scan(&runID); err != nil {
			return err
		}
		pl, e0 := json.Marshal(map[string]string{"run_id": runID})
		if e0 != nil {
			return e0
		}
		_, e := tx.Exec(ctx, `INSERT INTO work_jobs (org_id, kind, payload) VALUES ($1::uuid, 'tf_run', $2)`,
			oid, pl)
		return e
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, uid, "tf."+op, "tf_run", runID)
	s.json(w, 200, map[string]any{"runId": runID, "status": "pending", "op": op})
}

func (s *Server) GetRun(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	rid := chi.URLParam(r, "id")
	var st, op, errS, logtail string
	if err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT status, op, coalesce(error_summary, ''), coalesce(log_tail, '')
			FROM tf_runs WHERE id = $1::uuid AND org_id = $2::uuid
		`, rid, oid).Scan(&st, &op, &errS, &logtail)
	}); err != nil {
		s.json(w, 404, map[string]any{"error": "run not found"})
		return
	}
	s.json(w, 200, map[string]any{"id": rid, "status": st, "op": op, "error": errS, "logTail": logtail})
}
