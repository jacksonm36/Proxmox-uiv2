package api

import (
	"context"
	"encoding/json"
	"errors"
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
// Underlying PVE2 calls match https://pve.proxmox.com/pve-docs/api-viewer/ (e.g. /nodes, /nodes/{node}/qemu, /lxc).
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
			s.json(w, 502, map[string]any{
				"error": "list qemu on " + node + ": " + err.Error(),
			})
			return
		}
		for _, v := range vms {
			out = append(out, map[string]any{
				"node": node, "vmid": v.VMID, "name": v.Name, "status": v.Status, "type": v.Type, "kind": "qemu",
			})
		}
		lx, err := client.ListLxc(node)
		if err != nil {
			s.json(w, 502, map[string]any{
				"error": "list lxc on " + node + ": " + err.Error(),
			})
			return
		}
		for _, v := range lx {
			out = append(out, map[string]any{
				"node": node, "vmid": v.VMID, "name": v.Name, "status": v.Status, "type": "lxc", "kind": "lxc",
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
		// Common after CM_ENCRYPTION_KEY rotation or .env out of sync: AES-GCM Open fails with
		// "cipher: message authentication failed" — do not surface raw crypto details to the client.
		return nil, errors.New(
			"cannot decrypt the stored PVE API token: CM_ENCRYPTION_KEY on this server does not match the key " +
				"used when the connection was saved. An org admin must re-open Settings → Proxmox env, " +
				"re-enter the API token secret, and save (or restore the previous CM_ENCRYPTION_KEY). " +
				"Proxmox user must be like root@pam and token name a single id with no '!' in it (e.g. mytoken).",
		)
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

// GetPVEConnection GET: non-secret connection fields for the current org
func (s *Server) GetPVEConnection(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	if OrgRole(r.Context()) != "org_admin" && !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"org_admin required"})
		return
	}
	var sec *repo.PVESecret
	err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		var e error
		sec, e = repo.GetPVEByOrg(ctx, tx, oid)
		return e
	})
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if sec == nil || len(sec.EncToken) == 0 {
		s.json(w, 200, map[string]any{"configured": false})
		return
	}
	s.json(w, 200, map[string]any{
		"configured":   true,
		"baseUrl":      sec.BaseURL,
		"pveUser":      sec.PVEUser,
		"tokenId":      sec.TokenID,
		"resourcePool": sec.ResourcePool,
		"verifyTls":    sec.VerifyTLS,
	})
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
	if strings.TrimSpace(b.BaseURL) == "" || strings.TrimSpace(b.PVEUser) == "" {
		s.json(w, 400, jsonErr{"baseUrl and pveUser are required"})
		return
	}
	b.BaseURL = normalizePveBaseURL(b.BaseURL)
	b.PVEUser = normalizePveUserID(strings.TrimSpace(b.PVEUser))
	b.PoolPath = strings.TrimSpace(b.PoolPath)
	if tid, err := normalizeProxmoxTokenID(b.PVEUser, strings.TrimSpace(b.TokenID)); err != nil {
		s.json(w, 400, jsonErr{err.Error()})
		return
	} else {
		b.TokenID = tid
	}
	key, err := crypto.KeyFromHex(s.Cfg.EncryptionKey)
	if err != nil {
		s.json(w, 500, jsonErr{"server key"})
		return
	}

	var enc []byte
	secretIn := strings.TrimSpace(b.Secret)
	if secretIn == "" {
		var existing *repo.PVESecret
		err = db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
			var e error
			existing, e = repo.GetPVEByOrg(ctx, tx, oid)
			return e
		})
		if err != nil {
			s.json(w, 500, map[string]any{"error": err.Error()})
			return
		}
		if existing == nil || len(existing.EncToken) == 0 {
			s.json(w, 400, jsonErr{"API token secret is required for a new Proxmox connection"})
			return
		}
		tid := strings.TrimSpace(b.TokenID)
		if tid == "" {
			tid = existing.TokenID
		}
		if tid != existing.TokenID {
			s.json(w, 400, jsonErr{"enter the API token secret when changing token id"})
			return
		}
		enc = existing.EncToken
		b.TokenID = tid
	} else {
		if strings.TrimSpace(b.TokenID) == "" {
			s.json(w, 400, jsonErr{"tokenId is required when setting a new secret"})
			return
		}
		plain := []byte(strings.TrimSpace(b.TokenID) + "|" + secretIn)
		enc, err = crypto.Seal(plain, key)
		if err != nil {
			s.json(w, 500, jsonErr{"seal"})
			return
		}
	}

	if err := db.WithSession(r.Context(), s.Pool, sessionFor(r), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `
			INSERT INTO org_pve_secrets (org_id, base_url, pve_user, token_id, enc_token_secret, resource_pool, verify_tls, last_ok_at, last_error)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, now(), null)
			ON CONFLICT (org_id) DO UPDATE SET
				base_url=excluded.base_url, pve_user=excluded.pve_user, token_id=excluded.token_id,
				enc_token_secret=excluded.enc_token_secret, resource_pool=excluded.resource_pool,
				verify_tls=excluded.verify_tls, updated_at=now(), last_error=null
		`, oid, strings.TrimSpace(b.BaseURL), strings.TrimSpace(b.PVEUser), strings.TrimSpace(b.TokenID), enc, strings.TrimSpace(b.PoolPath), b.VerifyTLS)
		return e
	}); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "pve.connection.upsert", "org", oid)
	s.json(w, 200, map[string]any{"ok": true})
}

// normalizePveBaseURL strips a trailing /api2/json so the value matches the wiki model: base is
// https://host:8006 and paths like /api2/json/version are joined by the client (see
// https://pve.proxmox.com/wiki/Proxmox_VE_API#API_URL).
func normalizePveBaseURL(raw string) string {
	s := strings.TrimSpace(strings.TrimRight(raw, "/"))
	low := strings.ToLower(s)
	if strings.HasSuffix(low, "/api2/json") {
		s = s[:len(s)-len("/api2/json")]
		s = strings.TrimRight(s, "/")
	}
	return s
}

// normalizePveUserID trims a mistaken trailing "!" (users copy "root@pam!" from user!token strings).
func normalizePveUserID(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimSuffix(s, "!")
}

// normalizeProxmoxTokenID ensures we send PVEAPIToken=<pveUser>!<id>=<secret> with a single "!" between user and id.
// Users often paste the full "user!token" from the Proxmox UI into the token field; we strip a leading pveUser+"!".
func normalizeProxmoxTokenID(pveUser, tokenID string) (string, error) {
	if tokenID == "" {
		return "", nil
	}
	tokenID = strings.TrimSpace(tokenID)
	pveUser = strings.TrimSpace(pveUser)
	if pveUser != "" && strings.HasPrefix(tokenID, pveUser+"!") {
		tokenID = strings.TrimSpace(strings.TrimPrefix(tokenID, pveUser+"!"))
	}
	if strings.Contains(tokenID, "!") {
		return "", errors.New("token id must be one name with no '!' (e.g. 'test'). Put user in 'PVE user id' only. Proxmox full id looks like user@realm!name — the last part is the token id here")
	}
	if tokenID == "" {
		return "", errors.New("token id is empty after removing user! prefix; use the short name Proxmox shows after '!'")
	}
	return tokenID, nil
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
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "pve.power", "qemu", fmt.Sprintf("%s/%d", body.Node, body.VMID))
	s.json(w, 200, map[string]any{"ok": true})
}

// PostPVEConsole POST /api/v1/pve/console — vncproxy + noVNC URL (browser must reach PVE HTTPS port).
func (s *Server) PostPVEConsole(w http.ResponseWriter, r *http.Request) {
	oid := OrgID(r.Context())
	if oid == "" {
		s.json(w, 400, jsonErr{"set X-Cloudmanager-Org"})
		return
	}
	var body struct {
		Node string `json:"node"`
		VMID int    `json:"vmid"`
		Kind string `json:"kind"` // "qemu" or "lxc"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.json(w, 400, jsonErr{"invalid json"})
		return
	}
	if strings.TrimSpace(body.Node) == "" || body.VMID <= 0 {
		s.json(w, 400, jsonErr{"node and positive vmid required"})
		return
	}
	lxc := strings.ToLower(body.Kind) == "lxc"
	if body.Kind != "" && strings.ToLower(body.Kind) != "qemu" && !lxc {
		s.json(w, 400, jsonErr{"kind must be qemu or lxc"})
		return
	}
	if body.Kind == "" {
		lxc = false
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
	port, ticket, err := client.VncProxy(body.Node, body.VMID, lxc)
	if err != nil {
		s.json(w, 502, map[string]any{"error": err.Error()})
		return
	}
	u, err := client.NvcNoV1URL(body.Node, body.VMID, lxc, port, ticket)
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = repo.InsertAudit(r.Context(), s.Pool, oid, UserID(r.Context()), "pve.console",
		mapKind(body.Kind, lxc), fmt.Sprintf("%s/%d", body.Node, body.VMID))
	s.json(w, 200, map[string]any{"url": u, "port": port})
}

func mapKind(k string, lxc bool) string {
	if k != "" {
		return k
	}
	if lxc {
		return "lxc"
	}
	return "qemu"
}
