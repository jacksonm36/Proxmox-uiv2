package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	UserID          string
	OrgID           string
	OrgRole         string // org_admin | org_member
	IsPlatformAdmin bool
}

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "cloudmanager"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// WithSession sets Postgres GUCs for RLS (transaction-scoped).
func WithSession(ctx context.Context, pool *pgxpool.Pool, s *Session, fn func(ctx context.Context, tx pgx.Tx) error) error {
	if s == nil {
		return fmt.Errorf("nil session")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, _ = tx.Exec(ctx, "SET LOCAL app.is_platform_admin = "+boolToSQL(s.IsPlatformAdmin))
	_, _ = tx.Exec(ctx, "SET LOCAL app.org_id = "+quote(s.OrgID))
	_, _ = tx.Exec(ctx, "SET LOCAL app.org_role = "+quote(s.OrgRole))
	_, _ = tx.Exec(ctx, "SET LOCAL app.user_id = "+quote(s.UserID))

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func boolToSQL(b bool) string {
	if b {
		return "'true'"
	}
	return "'false'"
}

func quote(s string) string {
	if s == "" {
		return "''"
	}
	// escape single quotes
	r := "'"
	for _, c := range s {
		if c == '\'' {
			r += "''"
		} else {
			r += string(c)
		}
	}
	return r + "'"
}

// PlatformSession is used for MSP admin operations that may touch all orgs.
func PlatformSession(userID string) *Session {
	return &Session{
		UserID:          userID,
		OrgID:           "",
		OrgRole:         "org_member",
		IsPlatformAdmin: true,
	}
}

func OrgSession(userID, orgID, orgRole string) *Session {
	return &Session{
		UserID:          userID,
		OrgID:           orgID,
		OrgRole:         orgRole,
		IsPlatformAdmin: false,
	}
}
