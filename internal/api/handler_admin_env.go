package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cloudmanager/cloudmanager/internal/config"
	"github.com/cloudmanager/cloudmanager/internal/repo"
)

// Env keys a platform admin may edit in managed.env (see .env.example).
var adminEnvKeys = []string{
	"CM_HTTP_ADDR",
	"CM_DATABASE_URL",
	"CM_SESSION_SECRET",
	"CM_ENCRYPTION_KEY",
	"CM_REDIS_ADDR",
	"CM_BASE_URL",
	"CM_CORS_ORIGINS",
	"CM_TRUSTED_PROXIES",
	"CM_TERRAFORM_PATH",
	"CM_WORKDIR",
	"CM_WEB_ROOT",
	"CM_DISABLE_OIDC",
	"CM_DEV_BOOTSTRAP",
	"CM_OIDC_ISSUER",
	"CM_OIDC_CLIENT_ID",
	"CM_OIDC_CLIENT_SECRET",
	"CM_OIDC_REDIRECT_URL",
	"CM_DEV_BEARER",
	"CM_DEV_USER_EMAIL",
	"CM_WORKER_CONCURRENCY",
}

var adminEnvSecret = map[string]bool{
	"CM_SESSION_SECRET":     true,
	"CM_ENCRYPTION_KEY":     true,
	"CM_OIDC_CLIENT_SECRET": true,
	"CM_DEV_BEARER":         true,
}

func adminEnvAllowed(k string) bool {
	for _, x := range adminEnvKeys {
		if x == k {
			return true
		}
	}
	return false
}

// GetAdminEnv GET /api/v1/admin/env — read managed.env (redacted) — platform only.
func (s *Server) GetAdminEnv(w http.ResponseWriter, r *http.Request) {
	if !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"platform admin required"})
		return
	}
	path := s.Cfg.ManagedEnvFile
	if path == "" {
		s.json(w, 400, jsonErr{"set CM_WORKDIR or CM_MANAGED_ENV to enable a managed env file path"})
		return
	}
	m := make(map[string]string)
	fileExists := false
	b, err := os.ReadFile(path)
	if err == nil {
		fileExists = true
		m, err = config.ParseEnvFile(b)
		if err != nil {
			s.json(w, 500, map[string]any{"error": err.Error()})
			return
		}
	} else if !os.IsNotExist(err) {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	values := make(map[string]string)
	secretSet := make(map[string]bool)
	for _, k := range adminEnvKeys {
		v := m[k]
		if adminEnvSecret[k] {
			secretSet[k] = v != ""
			values[k] = ""
			continue
		}
		if k == "CM_DATABASE_URL" && v != "" {
			values[k] = maskDatabaseURL(v)
			continue
		}
		values[k] = v
	}
	s.json(w, 200, map[string]any{
		"path":            path,
		"fileExists":      fileExists,
		"values":          values,
		"secretSet":       secretSet,
		"keys":            adminEnvKeys,
		"restartRequired": "Restart the API and worker for these values to take full effect. Some are read only at start.",
	})
}

// PatchAdminEnv POST /api/v1/admin/env — partial update to managed.env — platform only.
func (s *Server) PatchAdminEnv(w http.ResponseWriter, r *http.Request) {
	if !IsPlatform(r.Context()) {
		s.json(w, 403, jsonErr{"platform admin required"})
		return
	}
	path := s.Cfg.ManagedEnvFile
	if path == "" {
		s.json(w, 400, jsonErr{"no managed env path (set CM_WORKDIR or CM_MANAGED_ENV)"})
		return
	}
	m := make(map[string]string)
	if b, err := os.ReadFile(path); err == nil {
		m, err = config.ParseEnvFile(b)
		if err != nil {
			s.json(w, 500, map[string]any{"error": err.Error()})
			return
		}
	} else if !os.IsNotExist(err) {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.json(w, 400, jsonErr{"invalid json"})
		return
	}
	if len(body) == 0 {
		s.json(w, 400, jsonErr{"empty body"})
		return
	}
	for k, v := range body {
		if !adminEnvAllowed(k) {
			s.json(w, 400, map[string]any{"error": "unknown or disallowed key: " + k})
			return
		}
		switch t := v.(type) {
		case string:
			if err := adminValidateValue(k, t); err != nil {
				s.json(w, 400, map[string]any{"error": err.Error(), "key": k})
				return
			}
			if t == "" {
				delete(m, k)
			} else {
				m[k] = t
			}
		case nil:
			// no-op
		default:
			s.json(w, 400, map[string]any{"error": "value must be string for key: " + k})
			return
		}
	}
	if err := config.WriteEnvFile(path, m); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_ = config.ApplyManagedEnvFile(path)
	_ = repo.InsertAudit(r.Context(), s.Pool, "", UserID(r.Context()), "admin.managed_env", "file", path)
	s.json(w, 200, map[string]any{
		"ok":       true,
		"path":     path,
		"notice":  "File written. Restart the API and worker to apply. CM_SESSION_SECRET / CM_ENCRYPTION_KEY changes can invalidate sessions or encrypted data.",
		"relogin": "You may need to sign in again if session secret changed in a future process.",
	})
}

func adminValidateValue(k, v string) error {
	if v == "" {
		return nil
	}
	switch k {
	case "CM_SESSION_SECRET":
		if len(v) < 32 {
			return fmt.Errorf("CM_SESSION_SECRET must be at least 32 bytes")
		}
	case "CM_ENCRYPTION_KEY":
		if len(v) != 64 {
			return fmt.Errorf("CM_ENCRYPTION_KEY must be exactly 64 hex characters (32 bytes AES key)")
		}
		if _, err := hex.DecodeString(v); err != nil {
			return fmt.Errorf("CM_ENCRYPTION_KEY: invalid hex")
		}
	}
	return nil
}

func maskDatabaseURL(u string) string {
	// password is between : (after user) and @
	i := strings.Index(u, "://")
	if i < 0 {
		return "***"
	}
	rest := u[i+3:]
	j := strings.LastIndex(rest, "@")
	if j < 0 {
		return u
	}
	userHost := rest[:j]
	if k := strings.LastIndexByte(userHost, ':'); k >= 0 {
		// :password
		return u[:i+3+k+1] + "***" + u[i+3+j:]
	}
	return u
}
