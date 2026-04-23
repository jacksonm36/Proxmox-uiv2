package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/repo"
	"github.com/jackc/pgx/v5"
)

func (s *Server) ListTemplates(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	sess := sessionFor(r)
	var out []map[string]any
	err := db.WithSession(r.Context(), s.Pool, sess, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, description, pve_node, template_vmid, coalesce(default_cloudinit, '')
			FROM template_catalog WHERE org_id = $1::uuid ORDER BY name
		`, oid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, desc, node, ci string
			var vmid int
			if err := rows.Scan(&id, &name, &desc, &node, &vmid, &ci); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"id": id, "name": name, "description": desc, "pveNode": node, "templateVmid": vmid, "defaultCloudinit": ci,
			})
		}
		return rows.Err()
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"templates": out})
}

func (s *Server) PostTemplate(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	if OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"forbidden"})
		return
	}
	var b struct {
		Name, Description, PVENode string
		TemplateVMID                 int    `json:"templateVmid"`
		DefaultCloudinit             string `json:"defaultCloudinit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.json(w, 400, jsonErr{"json"})
		return
	}
	sess := sessionFor(r)
	if err := db.WithSession(r.Context(), s.Pool, sess, func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `
			INSERT INTO template_catalog (org_id, name, description, pve_node, template_vmid, default_cloudinit)
			VALUES ($1::uuid, $2, $3, $4, $5, nullif($6, ''))
		`, oid, b.Name, b.Description, b.PVENode, b.TemplateVMID, b.DefaultCloudinit)
		return e
	}); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "template.create", "org", oid)
	s.json(w, 200, map[string]any{"ok": true})
}

func (s *Server) ListFlavors(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	sess := sessionFor(r)
	var out []map[string]any
	err := db.WithSession(r.Context(), s.Pool, sess, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, cpu_cores, memory_mb, disk_gb, coalesce(network_bridge, 'vmbr0')
			FROM flavors WHERE org_id = $1::uuid ORDER BY name
		`, oid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, br string
			var c, m, d int
			if err := rows.Scan(&id, &name, &c, &m, &d, &br); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"id": id, "name": name, "cpuCores": c, "memoryMb": m, "diskGb": d, "networkBridge": br,
			})
		}
		return rows.Err()
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"flavors": out})
}

func (s *Server) PostFlavor(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	if OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"forbidden"})
		return
	}
	var b struct {
		Name string `json:"name"`
		CPU  int    `json:"cpuCores"`
		Mem  int    `json:"memoryMb"`
		Disk int    `json:"diskGb"`
		Br   string `json:"networkBridge"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.json(w, 400, jsonErr{"json"})
		return
	}
	if b.Br == "" {
		b.Br = "vmbr0"
	}
	if err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `
			INSERT INTO flavors (org_id, name, cpu_cores, memory_mb, disk_gb, network_bridge)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
		`, oid, b.Name, b.CPU, b.Mem, b.Disk, b.Br)
		return e
	}); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "flavor.create", "org", oid)
	s.json(w, 200, map[string]any{"ok": true})
}

func (s *Server) ListAudit(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	var out []map[string]any
	err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, action, coalesce(target_type, ''), coalesce(target_id, ''), created_at
			FROM audit_log WHERE org_id = $1::uuid ORDER BY created_at DESC LIMIT 200
		`, oid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var a, tt, tid string
			var ct interface{}
			if err := rows.Scan(&id, &a, &tt, &tid, &ct); err != nil {
				return err
			}
			out = append(out, map[string]any{"id": id, "action": a, "targetType": tt, "targetId": tid, "at": ct})
		}
		return rows.Err()
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"items": out})
}

func (s *Server) ListOrgs(w http.ResponseWriter, r *http.Request) {
	uid := UserID(r.Context())
	if !IsPlatform(r.Context()) {
		orgs, _ := repo.ListOrgsForUser(r.Context(), s.Pool, uid)
		s.json(w, 200, map[string]any{"orgs": orgs})
		return
	}
	var out []map[string]any
	err := db.WithSession(r.Context(), s.Pool, db.PlatformSession(uid), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text, name, slug, max_vms FROM orgs ORDER BY name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, n, sl string
			var m int
			if err := rows.Scan(&id, &n, &sl, &m); err != nil {
				return err
			}
			out = append(out, map[string]any{"id": id, "name": n, "slug": sl, "maxVms": m})
		}
		return rows.Err()
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"orgs": out})
}

func sessionFor(r *http.Request) *db.Session {
	return &db.Session{
		UserID:          UserID(r.Context()),
		OrgID:           OrgID(r.Context()),
		OrgRole:         OrgRole(r.Context()),
		IsPlatformAdmin: IsPlatform(r.Context()),
	}
}
