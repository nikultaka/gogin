package apiclient

import "time"

// CreateClientRequest represents a client creation request
type CreateClientRequest struct {
	Name         string   `json:"name" binding:"required"`
	Description  string   `json:"description"`
	RedirectURIs []string `json:"redirect_uris" binding:"required"`
	Scopes       []string `json:"scopes" binding:"required"`
	GrantTypes   []string `json:"grant_types" binding:"required"`
	IsPublic     bool     `json:"is_public"`
}

// UpdateClientRequest represents a client update request
type UpdateClientRequest struct {
	Name         string   `json:"name" binding:"required"`
	Description  string   `json:"description"`
	RedirectURIs []string `json:"redirect_uris" binding:"required"`
	Scopes       []string `json:"scopes" binding:"required"`
	GrantTypes   []string `json:"grant_types" binding:"required"`
}

// ClientResponse represents a client response
type ClientResponse struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret,omitempty"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	RedirectURIs []string  `json:"redirect_uris"`
	Scopes       []string  `json:"scopes"`
	GrantTypes   []string  `json:"grant_types"`
	IsPublic     bool      `json:"is_public"`
	IsActive     bool      `json:"is_active"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ClientsListResponse represents a paginated list of clients
type ClientsListResponse struct {
	Clients    []*ClientResponse `json:"clients"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalPages int               `json:"total_pages"`
}
