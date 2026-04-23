package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTPAddr       string        `yaml:"http_addr"`
	DatabaseURL    string        `yaml:"database_url"`
	RedisAddr      string        `yaml:"redis_addr"`
	SessionSecret  string        `yaml:"session_secret"`
	EncryptionKey  string        `yaml:"encryption_key"` // 64 hex chars = 32 bytes AES-256
	OIDCIssuer     string        `yaml:"oidc_issuer"`
	OIDCClientID   string        `yaml:"oidc_client_id"`
	OIDCClientSec  string        `yaml:"oidc_client_secret"`
	OIDCRedirectURL string       `yaml:"oidc_redirect_url"`
	BaseURL        string        `yaml:"base_url"`
	CORSOrigins    []string      `yaml:"cors_origins"`
	TrustedProxies []string      `yaml:"trusted_proxies"`
	DisableOIDC    bool          `yaml:"disable_oidc"`
	DevBootstrap   bool          `yaml:"dev_bootstrap"`
	DevUserEmail   string        `yaml:"dev_user_email"`
	Workdir        string        `yaml:"workdir"`
	TerraformPath  string        `yaml:"terraform_path"`
	WorkerConc     int           `yaml:"worker_concurrency"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

func Load(path string) (*Config, error) {
	c := &Config{
		HTTPAddr:        "0.0.0.0:8080",
		BaseURL:         "http://localhost:8080",
		Workdir:         "/var/lib/cloudmanager",
		TerraformPath:   "terraform",
		WorkerConc:      2,
		RequestTimeout:  60 * time.Second,
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err == nil && len(b) > 0 {
			_ = yaml.Unmarshal(b, c)
		}
	}
	applyEnv(c)
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("database_url required (env CM_DATABASE_URL)")
	}
	if len(c.SessionSecret) < 32 {
		return nil, fmt.Errorf("session_secret must be at least 32 bytes (CM_SESSION_SECRET)")
	}
	if len(c.EncryptionKey) != 64 {
		return nil, fmt.Errorf("encryption_key must be 64 hex chars (CM_ENCRYPTION_KEY)")
	}
	return c, nil
}

func applyEnv(c *Config) {
	if v := os.Getenv("CM_HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv("CM_DATABASE_URL"); v != "" {
		c.DatabaseURL = v
	}
	if v := os.Getenv("CM_REDIS_ADDR"); v != "" {
		c.RedisAddr = v
	}
	if v := os.Getenv("CM_SESSION_SECRET"); v != "" {
		c.SessionSecret = v
	}
	if v := os.Getenv("CM_ENCRYPTION_KEY"); v != "" {
		c.EncryptionKey = v
	}
	if v := os.Getenv("CM_OIDC_ISSUER"); v != "" {
		c.OIDCIssuer = v
	}
	if v := os.Getenv("CM_OIDC_CLIENT_ID"); v != "" {
		c.OIDCClientID = v
	}
	if v := os.Getenv("CM_OIDC_CLIENT_SECRET"); v != "" {
		c.OIDCClientSec = v
	}
	if v := os.Getenv("CM_OIDC_REDIRECT_URL"); v != "" {
		c.OIDCRedirectURL = v
	}
	if v := os.Getenv("CM_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("CM_CORS_ORIGINS"); v != "" {
		c.CORSOrigins = splitTrim(v)
	}
	if v := os.Getenv("CM_TRUSTED_PROXIES"); v != "" {
		c.TrustedProxies = splitTrim(v)
	}
	if os.Getenv("CM_DISABLE_OIDC") == "1" || strings.EqualFold(os.Getenv("CM_DISABLE_OIDC"), "true") {
		c.DisableOIDC = true
	}
	if os.Getenv("CM_DEV_BOOTSTRAP") == "1" {
		c.DevBootstrap = true
	}
	if v := os.Getenv("CM_DEV_USER_EMAIL"); v != "" {
		c.DevUserEmail = v
	}
	if v := os.Getenv("CM_WORKDIR"); v != "" {
		c.Workdir = v
	}
	if v := os.Getenv("CM_TERRAFORM_PATH"); v != "" {
		c.TerraformPath = v
	}
	if v := os.Getenv("CM_WORKER_CONCURRENCY"); v != "" {
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			c.WorkerConc = n
		}
	}
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
