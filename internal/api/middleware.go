package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/cloudmanager/cloudmanager/internal/auth"
	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/repo"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Middleware struct {
	Pool   *pgxpool.Pool
	Signer *auth.Signer
	Dev    bool
	DevKey string
}

// Auth cookie/Bearer JWT; optional `cm_` API keys (bcrypt of full key stored).
func (m *Middleware) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skipAuthPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if tok, ok := m.extractToken(r); ok {
			// 1) API key: cm_...
			if auth.IsAPIKeyToken(tok) {
				if s, ok2 := m.authAPIKey(r.Context(), w, r, tok); ok2 {
					next.ServeHTTP(w, s)
					return
				}
				return
			}
			// 2) dev bearer shortcut
			if m.Dev && m.DevKey != "" && tok == m.DevKey {
				if s, ok2 := m.authDevUser(r, tok); ok2 {
					next.ServeHTTP(w, s)
					return
				}
			}
			// 3) session JWT
			if s, ok2 := m.authJWT(w, r, tok); ok2 {
				next.ServeHTTP(w, s)
				return
			}
		}
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
}

func skipAuthPath(p string) bool {
	switch p {
	case "/health", "/api/v1/auth/callback", "/api/v1/auth/login", "/api/v1/auth/dev", "/favicon.ico", "/":
		return true
	}
	if strings.HasPrefix(p, "/assets/") {
		return true
	}
	return strings.EqualFold(p, "/index.html")
}

func (m *Middleware) authAPIKey(_ context.Context, w http.ResponseWriter, r *http.Request, tok string) (*http.Request, bool) {
	oid := r.Header.Get(auth.HeaderOrg)
	if oid == "" {
		http.Error(w, `{"error":"missing X-Cloudmanager-Org"}`, http.StatusBadRequest)
		return nil, false
	}
	prefix, _, ok := parseAPIKey(tok)
	if !ok {
		return nil, false
	}
	// we stored: bcrypt( full token string including cm_)
	rows, err := m.Pool.Query(r.Context(), `
		SELECT key_hash, user_id::text, org_id::text, expires_at
		FROM api_keys
		WHERE key_prefix = $1
		ORDER BY created_at DESC
	`, prefix)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	for rows.Next() {
		var h []byte
		var uid, o string
		var exp *time.Time
		if err := rows.Scan(&h, &uid, &o, &exp); err != nil {
			continue
		}
		if exp != nil && time.Now().After(*exp) {
			continue
		}
		if o != oid {
			continue
		}
		if err := bcrypt.CompareHashAndPassword(h, []byte(tok)); err != nil {
			continue
		}
		role, err := repo.GetMembershipRole(r.Context(), m.Pool, uid, oid)
		if err != nil {
			continue
		}
		_, em, _ := getUserByID(m.Pool, r.Context(), uid)
		return r.WithContext(WithAuth(r.Context(), uid, em, oid, role, false)), true
	}
	return nil, false
}

func (m *Middleware) authDevUser(r *http.Request, _ string) (*http.Request, bool) {
	uid, isPlat, err := repo.GetUserByEmail(r.Context(), m.Pool, "dev@local.invalid")
	if err != nil || uid == "" {
		return nil, false
	}
	orgs, _ := repo.ListOrgsForUser(r.Context(), m.Pool, uid)
	oid := r.Header.Get(auth.HeaderOrg)
	role := "org_admin"
	if oid == "" && len(orgs) > 0 {
		oid = orgs[0].OrgID
		role = orgs[0].Role
	} else if oid != "" {
		rr, e := repo.GetMembershipRole(r.Context(), m.Pool, uid, oid)
		if e == nil {
			role = rr
		}
	}
	_, em, _, _ := repo.GetUserIDEmail(r.Context(), m.Pool, uid)
	return r.WithContext(WithAuth(r.Context(), uid, em, oid, role, isPlat)), true
}

func (m *Middleware) authJWT(w http.ResponseWriter, r *http.Request, tok string) (*http.Request, bool) {
	claims, err := m.Signer.Parse(tok)
	if err != nil {
		return nil, false
	}
	oid := r.Header.Get(auth.HeaderOrg)
	if !claims.IsPlatform && oid == "" {
		// first org
		orgs, err := repo.ListOrgsForUser(r.Context(), m.Pool, claims.UserID)
		if err != nil || len(orgs) == 0 {
			http.Error(w, `{"error":"no org membership; set X-Cloudmanager-Org"}`, http.StatusBadRequest)
			return nil, false
		}
		oid = orgs[0].OrgID
	}
	role := "org_member"
	if claims.IsPlatform {
		if oid != "" {
			role, _ = repo.GetMembershipRole(r.Context(), m.Pool, claims.UserID, oid)
		} else {
			role = "org_member"
		}
	} else {
		role, err = repo.GetMembershipRole(r.Context(), m.Pool, claims.UserID, oid)
		if err != nil {
			http.Error(w, `{"error":"forbidden org"}`, http.StatusForbidden)
			return nil, false
		}
	}
	return r.WithContext(WithAuth(r.Context(), claims.UserID, claims.Email, oid, role, claims.IsPlatform)), true
}

func (m *Middleware) extractToken(r *http.Request) (string, bool) {
	if t, ok := auth.ReadBearer(r); ok {
		return t, true
	}
	if c, err := r.Cookie(auth.CookieName); err == nil && c != nil && c.Value != "" {
		return c.Value, true
	}
	return "", false
}

func getUserByID(pool *pgxpool.Pool, ctx context.Context, id string) (string, string, error) {
	var e string
	var n *string
	err := pool.QueryRow(ctx, `SELECT email, name FROM users WHERE id = $1::uuid`, id).Scan(&e, &n)
	return id, e, err
}

// parse: cm_ + 8 char prefix (hex) + 32+ secret hex
func parseAPIKey(tok string) (prefix, rest string, ok bool) {
	if !strings.HasPrefix(tok, "cm_") {
		return "", "", false
	}
	rest = tok[3:]
	if len(rest) < 10 {
		return "", "", false
	}
	return rest[:8], rest[8:], true
}

// Hash API key for storage (for lookup) + bcrypt full
func NewAPIKeyMaterial() (full, prefix8 string, err error) {
	p := make([]byte, 4)
	_, e := rand.Read(p)
	if e != nil {
		return "", "", e
	}
	s := make([]byte, 24)
	if _, e := rand.Read(s); e != nil {
		return "", "", e
	}
	prefix8 = hex.EncodeToString(p)
	full = "cm_" + prefix8 + hex.EncodeToString(s)
	return full, prefix8, nil
}

func hashAPIKey(full string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(full), bcrypt.DefaultCost)
}

// VerifyAPIKey is used in tests
func VerifyAPIKey(hash []byte, full string) bool {
	return bcrypt.CompareHashAndPassword(hash, []byte(full)) == nil
}

func (m *Middleware) DBSession() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := &db.Session{
				UserID:          UserID(r.Context()),
				OrgID:           OrgID(r.Context()),
				OrgRole:         OrgRole(r.Context()),
				IsPlatformAdmin: IsPlatform(r.Context()),
			}
			_ = sess
			next.ServeHTTP(w, r)
		})
	}
}
