package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Org struct {
	ID, Name, Slug string
	MaxVMs         int
}

type Membership struct {
	OrgID  string
	Role   string
	Org    Org
}

func ListOrgsForUser(ctx context.Context, pool *pgxpool.Pool, userID string) ([]Membership, error) {
	rows, err := pool.Query(ctx, `
		SELECT o.id::text, o.name, o.slug, o.max_vms, m.role
		FROM org_memberships m
		JOIN orgs o ON o.id = m.org_id
		WHERE m.user_id = $1::uuid
		ORDER BY o.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.Org.ID, &m.Org.Name, &m.Org.Slug, &m.Org.MaxVMs, &m.Role); err != nil {
			return nil, err
		}
		m.OrgID = m.Org.ID
		out = append(out, m)
	}
	return out, rows.Err()
}

type PVESecret struct {
	OrgID         string
	BaseURL       string
	PVEUser       string
	TokenID       string
	EncToken      []byte
	ResourcePool  string
	VerifyTLS     bool
}

func GetPVEByOrg(ctx context.Context, tx pgx.Tx, orgID string) (*PVESecret, error) {
	var s PVESecret
	err := tx.QueryRow(ctx, `
		SELECT org_id::text, base_url, pve_user, coalesce(token_id, ''), enc_token_secret,
			coalesce(resource_pool, ''), verify_tls
		FROM org_pve_secrets
		WHERE org_id = $1::uuid
	`, orgID).Scan(&s.OrgID, &s.BaseURL, &s.PVEUser, &s.TokenID, &s.EncToken, &s.ResourcePool, &s.VerifyTLS)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func InsertAudit(ctx context.Context, pool *pgxpool.Pool, orgID, userID, action, targetType, targetID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SET LOCAL app.is_platform_admin = 'true'`); err != nil {
		return err
	}
	var org any = orgID
	if orgID == "" {
		org = nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (org_id, actor_user_id, action, target_type, target_id, meta)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, '{}'::jsonb)
	`, org, userID, action, nullStr(targetType), nullStr(targetID))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func GetUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (id string, isPlatform bool, err error) {
	err = pool.QueryRow(ctx, `SELECT id::text, is_platform_admin FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&id, &isPlatform)
	return
}

func GetUserIDEmail(ctx context.Context, pool *pgxpool.Pool, userID string) (id, email string, isPlatform bool, err error) {
	err = pool.QueryRow(ctx, `SELECT id::text, email, is_platform_admin FROM users WHERE id = $1::uuid`, userID).
		Scan(&id, &email, &isPlatform)
	return
}

func GetMembershipRole(ctx context.Context, pool *pgxpool.Pool, userID, orgID string) (string, error) {
	var role string
	err := pool.QueryRow(ctx, `
		SELECT role FROM org_memberships WHERE user_id = $1::uuid AND org_id = $2::uuid
	`, userID, orgID).Scan(&role)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("not a member of org")
	}
	return role, err
}
