package api

import (
	"context"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota + 1
	ctxEmail
	ctxOrgID
	ctxOrgRole
	ctxPlatform
)

func WithAuth(ctx context.Context, userID, email, orgID, orgRole string, platform bool) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, userID)
	ctx = context.WithValue(ctx, ctxEmail, email)
	if orgID != "" {
		ctx = context.WithValue(ctx, ctxOrgID, orgID)
		ctx = context.WithValue(ctx, ctxOrgRole, orgRole)
	}
	ctx = context.WithValue(ctx, ctxPlatform, platform)
	return ctx
}

func UserID(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}
func Email(ctx context.Context) string { v, _ := ctx.Value(ctxEmail).(string); return v }
func OrgID(ctx context.Context) string { v, _ := ctx.Value(ctxOrgID).(string); return v }
func OrgRole(ctx context.Context) string { v, _ := ctx.Value(ctxOrgRole).(string); return v }
func IsPlatform(ctx context.Context) bool { v, _ := ctx.Value(ctxPlatform).(bool); return v }
