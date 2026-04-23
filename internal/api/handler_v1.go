package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudmanager/cloudmanager/internal/auth"
	"github.com/cloudmanager/cloudmanager/internal/config"
	"github.com/cloudmanager/cloudmanager/internal/crypto"
	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/pve"
	"github.com/cloudmanager/cloudmanager/internal/repo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	Pool   *pgxpool.Pool
	Signer *auth.Signer
	Cfg    *config.Config
}

type jsonErr struct{ Error string }

func (s *Server) json(w http.ResponseWriter, st int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(st)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	uid := UserID(r.Context())
	if uid == "" {
		s.json(w, 401, jsonErr{"unauthorized"})
		return
	}
	orgs, _ := repo.ListOrgsForUser(r.Context(), s.Pool, uid)
	_, em, isPl, _ := repo.GetUserIDEmail(r.Context(), s.Pool, uid)
	orgList := make([]map[string]any, 0, len(orgs))
	for _, o := range orgs {
		orgList = append(orgList, map[string]any{
			"orgId": o.OrgID, "role": o.Role,
			"org":   map[string]any{"id": o.Org.ID, "name": o.Org.Name, "slug": o.Org.Slug, "maxVms": o.Org.MaxVMs},
		})
	}
	s.json(w, 200, map[string]any{
		"user":        map[string]any{"id": uid, "email": em, "isPlatformAdmin": isPl},
		"orgs":        orgList,
		"orgIdHeader": OrgID(r.Context()),
		"orgRole":     OrgRole(r.Context()),
	})
}

func (s *Server) DevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.DevBootstrap {
		s.json(w, 403, jsonErr{"dev login disabled"})
		return
	}
	uid, isPl, err := repo.GetUserByEmail(r.Context(), s.Pool, "dev@local.invalid")
	if err != nil || uid == "" {
		s.json(w, 500, jsonErr{"run bootstrap first"})
		return
	}
	claims := &auth.SessionClaims{UserID: uid, Email: "dev@local.invalid", IsPlatform: isPl}
	tos, err := s.Signer.Issue(claims, 24*time.Hour)
	if err != nil {
		s.json(w, 500, jsonErr{"token"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    tos,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
		Secure:   r.TLS != nil,
	})
	rel := r.URL.Query().Get("next")
	if rel == "" {
		rel = "/"
	}
	w.Header().Set("Location", rel)
	w.WriteHeader(http.StatusFound)
}

// PVEVMs GET /api/v1/pve/vms?node= optional
func (s *Server) PVEVMs(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	client, err := s.pveForOrg(r.Context(), oid)
	if err != nil {
		s.json(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if err := client.VerifyReachability(); err != nil {
		s.json(w, 502, map[string]any{"error": "pve unreachable: " + err.Error()})
		return
	}
	nodes, err := client.ListNodes()
	if err != nil {
		s.json(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if n := r.URL.Query().Get("node"); n != "" {
		var nn []string
		for _, x := range nodes {
			if x == n {
				nn = append(nn, x)
			}
		}
		if len(nn) > 0 {
			nodes = nn
		}
	}
	out := make([]any, 0, 64)
	for _, node := range nodes {
		vms, err := client.ListQemu(node)
		if err != nil {
			continue
		}
		for _, v := range vms {
			out = append(out, map[string]any{
				"node": node, "vmid": v.VMID, "name": v.Name, "status": v.Status, "type": v.Type,
			})
		}
	}
	s.json(w, 200, map[string]any{"vms": out, "nodes": nodes})
}

func (s *Server) pveForOrg(ctx context.Context, orgID string) (*pve.Client, error) {
	key, err := crypto.KeyFromHex(s.Cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_, _ = tx.Exec(ctx, "SET LOCAL app.is_platform_admin = 'true'")
	_, _ = tx.Exec(ctx, "SET LOCAL app.org_id = "+sqlQuote(orgID))
	sec, err := repo.GetPVEByOrg(ctx, tx, orgID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if sec == nil || len(sec.EncToken) == 0 {
		return nil, fmt.Errorf("no pve connection configured for org (POST /api/v1/pve/connection)")
	}
	raw, err := crypto.Open(sec.EncToken, key)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid pve token format")
	}
	return pve.NewClient(sec.BaseURL, sec.PVEUser, parts[0], parts[1], sec.VerifyTLS), nil
}

func sqlQuote(s string) string {
	b := &strings.Builder{}
	b.WriteString("'")
	for _, c := range s {
		if c == '\'' {
			b.WriteString("''")
		} else {
			b.WriteRune(c)
		}
	}
	b.WriteString("'")
	return b.String()
}

// PVEConnection POST: save pve
type PVEConnectionBody struct {
	BaseURL  string `json:"baseUrl"`
	PVEUser  string `json:"pveUser"`
	TokenID  string `json:"tokenId"`
	Secret   string `json:"secret"`
	PoolPath string `json:"resourcePool"`
	VerifyTLS bool  `json:"verifyTls"`
}

func (s *Server) PostPVEConnection(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	if OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"org_admin required"})
		return
	}
	var b PVEConnectionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.json(w, 400, jsonErr{"invalid json"})
		return
	}
	key, err := crypto.KeyFromHex(s.Cfg.EncryptionKey)
	if err != nil {
		s.json(w, 500, jsonErr{"server key"})
		return
	}
	plain := []byte(b.TokenID + "|" + b.Secret)
	enc, err := crypto.Seal(plain, key)
	if err != nil {
		s.json(w, 500, jsonErr{"seal"})
		return
	}
	if err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `
			INSERT INTO org_pve_secrets (org_id, base_url, pve_user, token_id, enc_token_secret, resource_pool, verify_tls, last_ok_at, last_error)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, now(), null)
			ON CONFLICT (org_id) DO UPDATE SET
				base_url=excluded.base_url, pve_user=excluded.pve_user, token_id=excluded.token_id,
				enc_token_secret=excluded.enc_token_secret, resource_pool=excluded.resource_pool,
				verify_tls=excluded.verify_tls, updated_at=now(), last_error=null
		`, oid, b.BaseURL, b.PVEUser, b.TokenID, enc, b.PoolPath, b.VerifyTLS)
		return e
	}); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "pve.connection.upsert", "org", oid)
	s.json(w, 200, map[string]any{"ok": true})
}

func (s *Server) PVEPower(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	var body struct {
		Node   string `json:"node"`
		VMID   int    `json:"vmid"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.json(w, 400, jsonErr{"json"})
		return
	}
	client, err := s.pveForOrg(r.Context(), oid)
	if err != nil {
		s.json(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if err := client.SetPower(body.Node, body.VMID, body.Action); err != nil {
		s.json(w, 502, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(s.Pool, r.Context(), oid, UserID(r.Context()), "pve.power", "qemu", fmt.Sprintf("%s/%d", body.Node, body.VMID))
	s.json(w, 200, map[string]any{"ok": true})
}
