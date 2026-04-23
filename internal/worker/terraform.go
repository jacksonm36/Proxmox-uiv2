package worker

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunOnce claims one pending run and executes terraform (apply or plan).
func RunOnce(ctx context.Context, pool *pgxpool.Pool, tfPath, workdir string) error {
	var runID, orgID, wsID, op, statePath, bundlePath string
	err := pool.QueryRow(ctx, `
		WITH cte AS (
			SELECT id FROM tf_runs WHERE status = 'pending' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE tf_runs r SET status = 'running', started_at = now()
		FROM cte WHERE r.id = cte.id
		RETURNING r.id::text, r.org_id::text, r.workspace_id::text, r.op, r.state_path,
			(SELECT bundle_path FROM tf_config_versions c WHERE c.id = r.config_version_id)
	`).Scan(&runID, &orgID, &wsID, &op, &statePath, &bundlePath)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if bundlePath == "" {
		_, _ = pool.Exec(ctx, `UPDATE tf_runs SET status = 'failed', error_summary = 'no bundle', finished_at = now() WHERE id = $1::uuid`, runID)
		return nil
	}
	tmp, err := os.MkdirTemp(workdir, "tf-"+runID+"-")
	if err != nil {
		_, _ = pool.Exec(ctx, `UPDATE tf_runs SET status = 'failed', error_summary = $1, finished_at = now() WHERE id = $2::uuid`, err.Error(), runID)
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck
	if err := extractTGZ(bundlePath, tmp); err != nil {
		_, _ = pool.Exec(ctx, `UPDATE tf_runs SET status = 'failed', error_summary = $1, finished_at = now() WHERE id = $2::uuid`, err.Error(), runID)
		return err
	}
	_ = os.Setenv("TF_IN_AUTOMATION", "1")
	if statePath != "" {
		_ = os.MkdirAll(filepath.Dir(statePath), 0o750)
		_ = os.Setenv("TF_DATA_DIR", filepath.Join(tmp, ".terraform"))
	}
	logf := filepath.Join(tmp, "terraform.log")
	logFile, _ := os.Create(logf)
	defer logFile.Close() //nolint:errcheck
	mw := io.MultiWriter(logFile)
	init := exec.CommandContext(ctx, tfPath, "init", "-input=false", "-no-color")
	init.Dir = tmp
	init.Stdout, init.Stderr = mw, mw
	if e := init.Run(); e != nil {
		fail(ctx, pool, runID, logf, e.Error())
		return e
	}
	var out []byte
	var tfErr error
	switch op {
	case "plan":
		cmd := exec.CommandContext(ctx, tfPath, "plan", "-input=false", "-no-color", "-lock=true")
		cmd.Dir = tmp
		out, tfErr = cmd.CombinedOutput()
		_, _ = mw.Write(out)
	case "apply", "destroy":
		args := []string{"apply", "-input=false", "-auto-approve", "-lock=true", "-no-color"}
		if op == "destroy" {
			args = []string{"destroy", "-input=false", "-auto-approve", "-lock=true", "-no-color"}
		}
		cmd := exec.CommandContext(ctx, tfPath, args...)
		cmd.Dir = tmp
		out, tfErr = cmd.CombinedOutput()
		_, _ = mw.Write(out)
	default:
		tfErr = fmt.Errorf("unknown op %s", op)
	}
	if tfErr != nil {
		fail(ctx, pool, runID, logf, tfErr.Error()+"\n"+tailStr(string(out), 4000))
		return tfErr
	}
	b, _ := os.ReadFile(logf)
	lt := tailStr(string(b), 32*1024)
	_, _ = pool.Exec(ctx, `UPDATE tf_runs SET status = 'succeeded', log_path = $1, log_tail = $2, finished_at = now() WHERE id = $3::uuid`, logf, lt, runID)
	_, _ = pool.Exec(ctx, `UPDATE work_jobs SET status = 'done', finished_at = now() WHERE kind = 'tf_run' AND payload->>'run_id' = $1`, runID)
	_ = wsID
	_ = orgID
	return nil
}

func fail(ctx context.Context, pool *pgxpool.Pool, runID, logf, msg string) {
	b, _ := os.ReadFile(logf)
	lt := tailStr(string(b), 32*1024)
	_, _ = pool.Exec(ctx, `UPDATE tf_runs SET status = 'failed', error_summary = $1, log_tail = $2, finished_at = now() WHERE id = $3::uuid`, msg, lt, runID)
}

func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func extractTGZ(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close() //nolint:errcheck
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(h.Name)
		if strings.HasPrefix(clean, "..") {
			continue
		}
		p := filepath.Join(dest, clean)
		if !strings.HasPrefix(filepath.Clean(p), filepath.Clean(dest)) {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(p, 0o750)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(p), 0o750)
			w, e := os.Create(p)
			if e != nil {
				return e
			}
			if _, e := io.Copy(w, tr); e != nil {
				_ = w.Close()
				return e
			}
			_ = w.Close()
		}
	}
}

// PollLoop ticks until ctx done.
func PollLoop(ctx context.Context, pool *pgxpool.Pool, tf, wd string, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = RunOnce(ctx, pool, tf, wd)
		}
	}
}

