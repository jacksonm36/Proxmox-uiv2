package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/repo"
	"github.com/jackc/pgx/v5"
)

type apiKeyRow struct {
	ID, Name, Prefix, Created, Expires string
}

func (s *Server) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	if OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"forbidden"})
		return
	}
	var out []apiKeyRow
	err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, key_prefix, created_at::text, coalesce(to_char(expires_at, 'YYYY-MM-DD'), '')
			FROM api_keys WHERE org_id = $1::uuid ORDER BY created_at DESC
		`, oid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, pfx, cr, ex string
			if err := rows.Scan(&id, &name, &pfx, &cr, &ex); err != nil {
				return err
			}
			out = append(out, apiKeyRow{ID: id, Name: name, Prefix: pfx, Created: cr, Expires: ex})
		}
		return rows.Err()
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"keys": out})
}

func (s *Server) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" || (OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context())) {
		s.json(w, 403, jsonErr{"forbidden"})
		return
	}
	var b struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.json(w, 400, jsonErr{"json"})
		return
	}
	full, prefix, err := NewAPIKeyMaterial()
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	h, err := hashAPIKey(full)
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	uid := UserID(r.Context())
	err = db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `
			INSERT INTO api_keys (org_id, user_id, name, key_prefix, key_hash)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		`, oid, uid, b.Name, prefix, h)
		return e
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, uid, "apikey.create", "org", oid)
	// Return secret once
	s.json(w, 200, map[string]any{"token": full, "keyPrefix": prefix, "name": b.Name, "message": "store token securely; it is not shown again"})
}
