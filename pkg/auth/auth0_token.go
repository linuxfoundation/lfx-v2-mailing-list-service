// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package auth provides OAuth2 token source implementations.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/oauth"
	"golang.org/x/oauth2"
)

const tokenExpiryLeeway = 60 * time.Second

// Auth0TokenSource implements oauth2.TokenSource using Auth0 SDK with private key JWT (client_assertion).
type Auth0TokenSource struct {
	ctx        context.Context
	authConfig *authentication.Authentication
	audience   string
	scope      string
}

// Token implements the oauth2.TokenSource interface.
func (a *Auth0TokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	body := oauth.LoginWithClientCredentialsRequest{
		Audience:        a.audience,
		ExtraParameters: map[string]string{"scope": a.scope},
	}

	tokenSet, err := a.authConfig.OAuth.LoginWithClientCredentials(ctx, body, oauth.IDTokenValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Auth0: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  tokenSet.AccessToken,
		TokenType:    tokenSet.TokenType,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn)*time.Second - tokenExpiryLeeway),
	}

	return token.WithExtra(map[string]any{"scope": tokenSet.Scope}), nil
}

// NewAuth0TokenSource creates a new Auth0 M2M token source using client assertion (private key JWT).
func NewAuth0TokenSource(ctx context.Context, authConfig *authentication.Authentication, audience, scope string) *Auth0TokenSource {
	return &Auth0TokenSource{
		ctx:        ctx,
		authConfig: authConfig,
		audience:   audience,
		scope:      scope,
	}
}