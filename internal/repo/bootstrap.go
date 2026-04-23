package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DevBootstrap creates a platform admin, org, and membership when tables are empty.
func DevBootstrap(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	email := "dev@local.invalid"
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL app.is_platform_admin = 'true'`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (oidc_sub, email, name, is_platform_admin)
		VALUES ($1, $2, $3, true)
	`, "dev-oidc-sub", email, "Dev Admin"); err != nil {
		return fmt.Errorf("dev user: %w", err)
	}
	var uid string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, email).Scan(&uid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO orgs (name, slug) VALUES ($1, $2)`, "Demo Customer", "demo"); err != nil {
		return err
	}
	var oid string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM orgs WHERE slug = 'demo'`).Scan(&oid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO org_memberships (org_id, user_id, role) VALUES ($1::uuid, $2::uuid, 'org_admin')
	`, oid, uid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
