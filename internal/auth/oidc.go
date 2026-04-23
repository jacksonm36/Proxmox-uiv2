package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDC struct {
	OAuth2       oauth2.Config
	Verifier     *oidc.IDTokenVerifier
	RedirectPath string
}

func NewOIDC(ctx context.Context, issuer, clientID, clientSecret, redirectURL, redirectPath string) (*OIDC, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &OIDC{
		RedirectPath: redirectPath,
		OAuth2: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		Verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}
