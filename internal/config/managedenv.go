package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveManagedEnvFile returns the path to the optional env overlay file.
// CM_MANAGED_ENV wins; otherwise <Workdir>/managed.env when Workdir is set.
func ResolveManagedEnvFile(c *Config) string {
	if p := os.Getenv("CM_MANAGED_ENV"); p != "" {
		return p
	}
	if c != nil && c.Workdir != "" {
		return filepath.Join(c.Workdir, "managed.env")
	}
	return ""
}

// ApplyManagedEnvFile loads KEY=... lines and calls os.Setenv. Missing file is ok.
func ApplyManagedEnvFile(path string) error {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	m, err := ParseEnvFile(b)
	if err != nil {
		return err
	}
	for k, v := range m {
		_ = os.Setenv(k, v)
	}
	return nil
}

// ParseEnvFile parses a minimal dotenv (no multiline).
func ParseEnvFile(b []byte) (map[string]string, error) {
	m := make(map[string]string)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "export ") {
			line = strings.TrimSpace(line[7:])
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		m[k] = v
	}
	return m, nil
}

// WriteEnvFile writes env lines sorted by key.
func WriteEnvFile(path string, m map[string]string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# Cloudmanager — written from admin UI. Restart the API worker after changes.\n")
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(quoteEnvValue(m[k]))
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func quoteEnvValue(v string) string {
	if v == "" {
		return `""`
	}
	if !strings.ContainsAny(v, " \t\n\"'#\\$") {
		return v
	}
	s := strings.ReplaceAll(v, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
