package models

import (
	"database/sql"
	"time"
)

// OAuthClient represents an OAuth 2.0 client application
type OAuthClient struct {
	ID               string         `json:"id" db:"id"`
	ClientID         string         `json:"client_id" db:"client_id"`
	ClientSecret     string         `json:"-" db:"client_secret"`
	Name             string         `json:"name" db:"name"`
	Description      sql.NullString `json:"description,omitempty" db:"description"`
	RedirectURIs     string         `json:"redirect_uris" db:"redirect_uris"` // JSON array
	Scopes           string         `json:"scopes" db:"scopes"` // Space-separated scopes
	GrantTypes       string         `json:"grant_types" db:"grant_types"` // Space-separated grant types
	IsPublic         bool           `json:"is_public" db:"is_public"` // Public client (no secret required)
	IsActive         bool           `json:"is_active" db:"is_active"`
	CreatedBy        string         `json:"created_by" db:"created_by"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
	DeletedAt        sql.NullTime   `json:"deleted_at,omitempty" db:"deleted_at"`
}

// OAuthToken represents an OAuth 2.0 access token
type OAuthToken struct {
	ID            string       `json:"id" db:"id"`
	AccessToken   string       `json:"access_token" db:"access_token"`
	RefreshToken  sql.NullString `json:"refresh_token,omitempty" db:"refresh_token"`
	TokenType     string       `json:"token_type" db:"token_type"` // Bearer
	ExpiresAt     time.Time    `json:"expires_at" db:"expires_at"`
	Scopes        string       `json:"scopes" db:"scopes"` // Space-separated scopes
	ClientID      string       `json:"client_id" db:"client_id"`
	UserID        sql.NullString `json:"user_id,omitempty" db:"user_id"`
	IsRevoked     bool         `json:"is_revoked" db:"is_revoked"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// OAuthAuthorizationCode represents an OAuth 2.0 authorization code
type OAuthAuthorizationCode struct {
	ID              string       `json:"id" db:"id"`
	Code            string       `json:"code" db:"code"`
	ClientID        string       `json:"client_id" db:"client_id"`
	UserID          string       `json:"user_id" db:"user_id"`
	RedirectURI     string       `json:"redirect_uri" db:"redirect_uri"`
	Scopes          string       `json:"scopes" db:"scopes"`
	CodeChallenge   sql.NullString `json:"code_challenge,omitempty" db:"code_challenge"`
	CodeChallengeMethod sql.NullString `json:"code_challenge_method,omitempty" db:"code_challenge_method"`
	ExpiresAt       time.Time    `json:"expires_at" db:"expires_at"`
	IsUsed          bool         `json:"is_used" db:"is_used"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
}

// IsExpired returns true if the authorization code is expired
func (c *OAuthAuthorizationCode) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// IsValid returns true if the authorization code is valid
func (c *OAuthAuthorizationCode) IsValid() bool {
	return !c.IsUsed && !c.IsExpired()
}
