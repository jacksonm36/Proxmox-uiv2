package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	CookieName = "cm_session"
	HeaderOrg  = "X-Cloudmanager-Org"
)

type SessionClaims struct {
	jwt.RegisteredClaims
	UserID    string   `json:"uid"`
	Email     string   `json:"em"`
	IsPlatform bool    `json:"pl"`
	OrgSlugs  []string `json:"orgslugs"` // for display; org ID resolved server-side
}

type Session struct {
	UserID   string
	Email    string
	OrgID    string
	OrgRole  string
	Platform bool
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

type Signer struct {
	secret []byte
}

func (s *Signer) Issue(claims *SessionClaims, ttl time.Duration) (string, error) {
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

func (s *Signer) Parse(token string) (*SessionClaims, error) {
	claims := &SessionClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func ReadBearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const p = "Bearer "
	if !strings.HasPrefix(h, p) {
		return "", false
	}
	return strings.TrimSpace(h[len(p):]), true
}

func IsAPIKeyToken(tok string) bool {
	return strings.HasPrefix(tok, "cm_")
}
