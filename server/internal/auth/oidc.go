package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"confero/internal/repository"
)

const (
	stateCookieName    = "oidc_state"
	verifierCookieName = "oidc_verifier"
	cookiePath         = "/"
)

// UserUpserter is implemented by repository.Queries.
type UserUpserter interface {
	UpsertUser(ctx context.Context, arg repository.UpsertUserParams) (repository.User, error)
	UpsertUserSettings(ctx context.Context, userID uuid.UUID) error
}

// OIDCHandler handles /auth/login, /auth/callback, and /auth/logout.
type OIDCHandler struct {
	provider     *gooidc.Provider
	oauth2Config oauth2.Config
	tokens       *TokenManager
	users        UserUpserter
	memberValue  string
	adminValue   string
	logger       *slog.Logger
}

// NewOIDCHandler discovers the OIDC provider and wires up the handler.
// Discovery is retried up to 10 times with a 3-second backoff so the server
// starts cleanly even when Keycloak is still initialising alongside it.
func NewOIDCHandler(
	ctx context.Context,
	issuerURL, clientID, clientSecret, redirectURL string,
	memberValue, adminValue string,
	tokens *TokenManager,
	users UserUpserter,
	logger *slog.Logger,
) (*OIDCHandler, error) {
	var provider *gooidc.Provider
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		provider, err = gooidc.NewProvider(ctx, issuerURL)
		if err == nil {
			break
		}
		if attempt < 10 {
			logger.Warn("OIDC provider not ready, retrying", "attempt", attempt, "err", err)
			time.Sleep(3 * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("oidc provider discovery: %w", err)
	}

	return &OIDCHandler{
		provider: provider,
		oauth2Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
		},
		tokens:      tokens,
		users:       users,
		memberValue: memberValue,
		adminValue:  adminValue,
		logger:      logger,
	}, nil
}

// Login redirects the browser to the IdP authorization endpoint.
func (h *OIDCHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomBase64(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier, challenge, err := pkce()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setShorttermCookie(w, stateCookieName, state)
	setShorttermCookie(w, verifierCookieName, verifier)

	url := h.oauth2Config.AuthCodeURL(state,
		oauth2.S256ChallengeOption(challenge),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback handles the OIDC code exchange and issues a session JWT.
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	state, err := r.Cookie(stateCookieName)
	if err != nil || state.Value == "" || state.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	verifierCookie, err := r.Cookie(verifierCookieName)
	if err != nil || verifierCookie.Value == "" {
		http.Error(w, "missing PKCE verifier", http.StatusBadRequest)
		return
	}

	clearCookie(w, stateCookieName)
	clearCookie(w, verifierCookieName)

	ctx := r.Context()
	token, err := h.oauth2Config.Exchange(ctx, r.URL.Query().Get("code"),
		oauth2.VerifierOption(verifierCookie.Value),
	)
	if err != nil {
		h.logger.Error("token exchange failed", "err", err)
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "missing id_token", http.StatusBadGateway)
		return
	}

	verifier := h.provider.Verifier(&gooidc.Config{ClientID: h.oauth2Config.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		h.logger.Error("id_token verification failed", "err", err)
		http.Error(w, "id_token invalid", http.StatusUnauthorized)
		return
	}

	var claims map[string]any
	if err = idToken.Claims(&claims); err != nil {
		http.Error(w, "claim parse error", http.StatusInternalServerError)
		return
	}

	groups := extractGroups(claims, OIDCClaimName)
	roles := DecodeRoles(groups, h.memberValue, h.adminValue)
	if !roles.Member {
		http.Error(w, "not a chair member", http.StatusForbidden)
		return
	}

	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if name == "" {
		name, _ = claims["preferred_username"].(string)
	}

	user, err := h.users.UpsertUser(ctx, repository.UpsertUserParams{
		OidcIssuer:  idToken.Issuer,
		OidcSubject: idToken.Subject,
		Email:       email,
		DisplayName: name,
	})
	if err != nil {
		h.logger.Error("user upsert failed", "err", err)
		http.Error(w, "user upsert failed", http.StatusInternalServerError)
		return
	}
	if err := h.users.UpsertUserSettings(ctx, user.ID); err != nil {
		h.logger.Error("user settings upsert failed", "err", err)
		http.Error(w, "user settings error", http.StatusInternalServerError)
		return
	}

	sc := SessionClaims{
		Email:   email,
		Name:    name,
		OIDCSub: idToken.Subject,
		Roles:   roles.ToStringSlice(),
	}
	sc.Subject = user.ID.String()
	sessionJWT, err := h.tokens.Issue(sc)
	if err != nil {
		h.logger.Error("token issue failed", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionJWT,
		Path:     cookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(tokenTTL.Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout clears the session cookie.
func (h *OIDCHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, cookieName)
	http.Redirect(w, r, "/", http.StatusFound)
}

func extractGroups(claims map[string]any, claimName string) []string {
	raw, ok := claims[claimName]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		var parsed []string
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
		return []string{v}
	}
	return nil
}

func setShorttermCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     cookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((5 * time.Minute).Seconds()),
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Value:   "",
		Path:    cookiePath,
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
}

func randomBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkce() (verifier, challenge string, err error) {
	v, err := randomBase64(32)
	if err != nil {
		return "", "", err
	}
	h := sha256.Sum256([]byte(v))
	c := base64.RawURLEncoding.EncodeToString(h[:])
	return v, c, nil
}
