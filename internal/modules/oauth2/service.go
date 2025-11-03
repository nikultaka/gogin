package oauth2

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"gogin/internal/clients"
	"gogin/internal/config"
	"gogin/internal/models"
	"gogin/internal/modules/redishelper"
	"gogin/internal/utils"

	"github.com/google/uuid"
)

// OAuth2Service handles OAuth2 business logic
type OAuth2Service struct {
	db          *clients.Database
	redisHelper *redishelper.RedisHelper
	jwtUtil     *utils.JWTUtil
	config      *config.Config
}

// NewOAuth2Service creates a new OAuth2 service
func NewOAuth2Service(db *clients.Database, redisHelper *redishelper.RedisHelper, jwtUtil *utils.JWTUtil, cfg *config.Config) *OAuth2Service {
	return &OAuth2Service{
		db:          db,
		redisHelper: redisHelper,
		jwtUtil:     jwtUtil,
		config:      cfg,
	}
}

// CreateAuthorizationCode creates an authorization code
func (s *OAuth2Service) CreateAuthorizationCode(userID string, req *AuthorizeRequest) (*models.OAuthAuthorizationCode, error) {
	// Verify client
	client, err := s.GetClientByClientID(req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client")
	}

	if !client.IsActive {
		return nil, fmt.Errorf("client is inactive")
	}

	// Verify redirect URI
	if !s.validateRedirectURI(client, req.RedirectURI) {
		return nil, fmt.Errorf("invalid redirect URI")
	}

	// Generate authorization code
	code := uuid.New().String()
	expiresAt := time.Now().Add(10 * time.Minute) // 10 minute expiry

	authCode := &models.OAuthAuthorizationCode{
		ID:          uuid.New().String(),
		Code:        code,
		ClientID:    req.ClientID,
		UserID:      userID,
		RedirectURI: req.RedirectURI,
		Scopes:      req.Scope,
		ExpiresAt:   expiresAt,
		IsUsed:      false,
	}

	if req.CodeChallenge != "" {
		authCode.CodeChallenge = sql.NullString{String: req.CodeChallenge, Valid: true}
		authCode.CodeChallengeMethod = sql.NullString{String: req.CodeChallengeMethod, Valid: true}
	}

	query := `
		INSERT INTO oauth_authorization_codes
		(id, code, client_id, user_id, redirect_uri, scopes, code_challenge, code_challenge_method, expires_at, is_used, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`

	_, err = s.db.Exec(query,
		authCode.ID,
		authCode.Code,
		authCode.ClientID,
		authCode.UserID,
		authCode.RedirectURI,
		authCode.Scopes,
		authCode.CodeChallenge,
		authCode.CodeChallengeMethod,
		authCode.ExpiresAt,
		authCode.IsUsed,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create authorization code: %w", err)
	}

	return authCode, nil
}

// ExchangeCodeForToken exchanges authorization code for access token
func (s *OAuth2Service) ExchangeCodeForToken(req *TokenRequest) (*TokenResponse, error) {
	// Get authorization code
	var authCode models.OAuthAuthorizationCode
	query := `
		SELECT id, code, client_id, user_id, redirect_uri, scopes,
		       code_challenge, code_challenge_method, expires_at, is_used, created_at
		FROM oauth_authorization_codes
		WHERE code = $1 AND is_used = FALSE
	`

	err := s.db.QueryRow(query, req.Code).Scan(
		&authCode.ID,
		&authCode.Code,
		&authCode.ClientID,
		&authCode.UserID,
		&authCode.RedirectURI,
		&authCode.Scopes,
		&authCode.CodeChallenge,
		&authCode.CodeChallengeMethod,
		&authCode.ExpiresAt,
		&authCode.IsUsed,
		&authCode.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("invalid authorization code")
	}

	// Verify code hasn't expired
	if authCode.IsExpired() {
		return nil, fmt.Errorf("authorization code expired")
	}

	// Verify client
	if authCode.ClientID != req.ClientID {
		return nil, fmt.Errorf("client mismatch")
	}

	// Verify redirect URI
	if authCode.RedirectURI != req.RedirectURI {
		return nil, fmt.Errorf("redirect URI mismatch")
	}

	// Verify PKCE if present
	if authCode.CodeChallenge.Valid {
		if !s.verifyPKCE(authCode.CodeChallenge.String, authCode.CodeChallengeMethod.String, req.CodeVerifier) {
			return nil, fmt.Errorf("invalid code verifier")
		}
	}

	// Mark code as used
	_, err = s.db.Exec("UPDATE oauth_authorization_codes SET is_used = TRUE WHERE id = $1", authCode.ID)
	if err != nil {
		return nil, err
	}

	// Get client for scope validation
	client, err := s.GetClientByClientID(req.ClientID)
	if err != nil {
		return nil, err
	}

	// Verify client secret if not public client
	if !client.IsPublic {
		if req.ClientSecret != client.ClientSecret {
			return nil, fmt.Errorf("invalid client secret")
		}
	}

	// Generate tokens
	scopes := strings.Split(authCode.Scopes, " ")
	return s.generateTokens(authCode.UserID, req.ClientID, scopes)
}

// ClientCredentialsGrant handles client credentials grant
func (s *OAuth2Service) ClientCredentialsGrant(req *TokenRequest) (*TokenResponse, error) {
	// Get and verify client
	client, err := s.GetClientByClientID(req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client")
	}

	if !client.IsActive {
		return nil, fmt.Errorf("client is inactive")
	}

	if req.ClientSecret != client.ClientSecret {
		return nil, fmt.Errorf("invalid client secret")
	}

	// Verify grant type is allowed
	if !strings.Contains(client.GrantTypes, "client_credentials") {
		return nil, fmt.Errorf("grant type not allowed")
	}

	// Use requested scope or default to client scopes
	scope := req.Scope
	if scope == "" {
		scope = client.Scopes
	}

	// Generate access token (no refresh token for client credentials)
	scopes := strings.Split(scope, " ")
	accessToken, _, err := s.jwtUtil.GenerateClientToken(
		req.ClientID,
		scopes,
		s.config.OAuth.AccessTokenExpiry,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	expiresAt := time.Now().Add(s.config.OAuth.AccessTokenExpiry)

	// Store token
	_, err = s.db.Exec(`
		INSERT INTO oauth_tokens (id, access_token, token_type, expires_at, scopes, client_id, is_revoked, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`, uuid.New().String(), accessToken, "Bearer", expiresAt, scope, req.ClientID, false)

	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(s.config.OAuth.AccessTokenExpiry.Seconds()),
		Scope:       scope,
	}, nil
}

// RefreshTokenGrant handles refresh token grant
func (s *OAuth2Service) RefreshTokenGrant(req *TokenRequest) (*TokenResponse, error) {
	// Validate refresh token
	claims, err := s.jwtUtil.ValidateToken(req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if token is revoked
	revoked, _ := s.redisHelper.IsTokenRevoked(claims.TokenID)
	if revoked {
		return nil, fmt.Errorf("refresh token has been revoked")
	}

	// Verify client
	if claims.ClientID != req.ClientID {
		return nil, fmt.Errorf("client mismatch")
	}

	// Generate new tokens
	return s.generateTokens(claims.UserID, req.ClientID, claims.Scopes)
}

// RevokeToken revokes an access or refresh token
func (s *OAuth2Service) RevokeToken(token string) error {
	// Validate token to get claims
	claims, err := s.jwtUtil.ValidateToken(token)
	if err != nil {
		return fmt.Errorf("invalid token")
	}

	// Add to revocation list
	expiresAt := claims.ExpiresAt.Time
	return s.redisHelper.RevokeToken(claims.TokenID, expiresAt)
}

// IntrospectToken introspects a token
func (s *OAuth2Service) IntrospectToken(token string) (*IntrospectResponse, error) {
	// Validate token
	claims, err := s.jwtUtil.ValidateToken(token)
	if err != nil {
		return &IntrospectResponse{Active: false}, nil
	}

	// Check if revoked
	revoked, _ := s.redisHelper.IsTokenRevoked(claims.TokenID)
	if revoked {
		return &IntrospectResponse{Active: false}, nil
	}

	return &IntrospectResponse{
		Active:    true,
		Scope:     strings.Join(claims.Scopes, " "),
		ClientID:  claims.ClientID,
		UserID:    claims.UserID,
		TokenType: "Bearer",
		ExpiresAt: claims.ExpiresAt.Unix(),
		IssuedAt:  claims.IssuedAt.Unix(),
	}, nil
}

// GetClientByClientID retrieves a client by client ID
func (s *OAuth2Service) GetClientByClientID(clientID string) (*models.OAuthClient, error) {
	var client models.OAuthClient
	query := `
		SELECT id, client_id, client_secret, name, description, redirect_uris,
		       scopes, grant_types, is_public, is_active, created_by, created_at, updated_at, deleted_at
		FROM oauth_clients
		WHERE client_id = $1 AND deleted_at IS NULL
	`

	err := s.db.QueryRow(query, clientID).Scan(
		&client.ID,
		&client.ClientID,
		&client.ClientSecret,
		&client.Name,
		&client.Description,
		&client.RedirectURIs,
		&client.Scopes,
		&client.GrantTypes,
		&client.IsPublic,
		&client.IsActive,
		&client.CreatedBy,
		&client.CreatedAt,
		&client.UpdatedAt,
		&client.DeletedAt,
	)

	if err != nil {
		return nil, err
	}

	return &client, nil
}

// Helper functions

func (s *OAuth2Service) generateTokens(userID, clientID string, scopes []string) (*TokenResponse, error) {
	// Generate access token
	accessToken, _, err := s.jwtUtil.GenerateAccessToken(
		userID,
		clientID,
		"",
		scopes,
		s.config.OAuth.AccessTokenExpiry,
	)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken, _, err := s.jwtUtil.GenerateRefreshToken(
		userID,
		clientID,
		s.config.OAuth.RefreshTokenExpiry,
	)
	if err != nil {
		return nil, err
	}

	// Store tokens
	expiresAt := time.Now().Add(s.config.OAuth.AccessTokenExpiry)
	_, err = s.db.Exec(`
		INSERT INTO oauth_tokens (id, access_token, refresh_token, token_type, expires_at, scopes, client_id, user_id, is_revoked, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`, uuid.New().String(), accessToken, refreshToken, "Bearer", expiresAt, strings.Join(scopes, " "), clientID, userID, false)

	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.config.OAuth.AccessTokenExpiry.Seconds()),
		RefreshToken: refreshToken,
		Scope:        strings.Join(scopes, " "),
	}, nil
}

func (s *OAuth2Service) validateRedirectURI(client *models.OAuthClient, redirectURI string) bool {
	// Simple validation - should be in client's allowed redirect URIs
	return strings.Contains(client.RedirectURIs, redirectURI)
}

func (s *OAuth2Service) verifyPKCE(challenge, method, verifier string) bool {
	if method == "S256" {
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		return computed == challenge
	}
	// Plain method
	return verifier == challenge
}
