package apiclient

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gogin/internal/clients"
	"gogin/internal/models"
	"gogin/internal/modules/redishelper"

	"github.com/google/uuid"
)

// APIClientService handles API client business logic
type APIClientService struct {
	db          *clients.Database
	redisHelper *redishelper.RedisHelper
}

// NewAPIClientService creates a new API client service
func NewAPIClientService(db *clients.Database, redisHelper *redishelper.RedisHelper) *APIClientService {
	return &APIClientService{
		db:          db,
		redisHelper: redisHelper,
	}
}

// CreateClient creates a new OAuth client
func (s *APIClientService) CreateClient(userID string, req *CreateClientRequest) (*ClientResponse, error) {
	clientID := s.generateClientID()
	clientSecret := s.generateClientSecret()

	redirectURIsJSON, _ := json.Marshal(req.RedirectURIs)
	scopes := strings.Join(req.Scopes, " ")
	grantTypes := strings.Join(req.GrantTypes, " ")

	id := uuid.New().String()
	query := `
		INSERT INTO oauth_clients
		(id, client_id, client_secret, name, description, redirect_uris, scopes, grant_types, is_public, is_active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := s.db.QueryRow(query,
		id,
		clientID,
		clientSecret,
		req.Name,
		req.Description,
		string(redirectURIsJSON),
		scopes,
		grantTypes,
		req.IsPublic,
		true,
		userID,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &ClientResponse{
		ID:           id,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Name:         req.Name,
		Description:  req.Description,
		RedirectURIs: req.RedirectURIs,
		Scopes:       req.Scopes,
		GrantTypes:   req.GrantTypes,
		IsPublic:     req.IsPublic,
		IsActive:     true,
		CreatedBy:    userID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// GetClient retrieves a client by ID
func (s *APIClientService) GetClient(id string) (*ClientResponse, error) {
	var client models.OAuthClient
	query := `
		SELECT id, client_id, client_secret, name, description, redirect_uris,
		       scopes, grant_types, is_public, is_active, created_by, created_at, updated_at
		FROM oauth_clients
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := s.db.QueryRow(query, id).Scan(
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
	)

	if err != nil {
		return nil, err
	}

	return s.toClientResponse(&client), nil
}

// ListClients lists all clients with pagination
func (s *APIClientService) ListClients(page, limit int) ([]*ClientResponse, int, error) {
	offset := (page - 1) * limit

	// Get total count
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM oauth_clients WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get clients
	query := `
		SELECT id, client_id, client_secret, name, description, redirect_uris,
		       scopes, grant_types, is_public, is_active, created_by, created_at, updated_at
		FROM oauth_clients
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var clients []*ClientResponse
	for rows.Next() {
		var client models.OAuthClient
		err := rows.Scan(
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
		)
		if err != nil {
			return nil, 0, err
		}
		clients = append(clients, s.toClientResponse(&client))
	}

	return clients, total, nil
}

// UpdateClient updates a client
func (s *APIClientService) UpdateClient(id string, req *UpdateClientRequest) (*ClientResponse, error) {
	redirectURIsJSON, _ := json.Marshal(req.RedirectURIs)
	scopes := strings.Join(req.Scopes, " ")
	grantTypes := strings.Join(req.GrantTypes, " ")

	query := `
		UPDATE oauth_clients
		SET name = $1, description = $2, redirect_uris = $3, scopes = $4, grant_types = $5, updated_at = NOW()
		WHERE id = $6 AND deleted_at IS NULL
	`

	result, err := s.db.Exec(query,
		req.Name,
		req.Description,
		string(redirectURIsJSON),
		scopes,
		grantTypes,
		id,
	)

	if err != nil {
		return nil, err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("client not found")
	}

	return s.GetClient(id)
}

// DeleteClient soft deletes a client
func (s *APIClientService) DeleteClient(id string) error {
	query := `UPDATE oauth_clients SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// RegenerateSecret generates a new client secret
func (s *APIClientService) RegenerateSecret(id string) (string, error) {
	newSecret := s.generateClientSecret()

	query := `UPDATE oauth_clients SET client_secret = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	result, err := s.db.Exec(query, newSecret, id)
	if err != nil {
		return "", err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", fmt.Errorf("client not found")
	}

	return newSecret, nil
}

// UpdateStatus updates client status
func (s *APIClientService) UpdateStatus(id string, isActive bool) error {
	query := `UPDATE oauth_clients SET is_active = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	result, err := s.db.Exec(query, isActive, id)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// Helper functions

func (s *APIClientService) generateClientID() string {
	return fmt.Sprintf("client_%s", uuid.New().String())
}

func (s *APIClientService) generateClientSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *APIClientService) toClientResponse(client *models.OAuthClient) *ClientResponse {
	var redirectURIs []string
	json.Unmarshal([]byte(client.RedirectURIs), &redirectURIs)

	scopes := strings.Split(client.Scopes, " ")
	grantTypes := strings.Split(client.GrantTypes, " ")

	description := ""
	if client.Description.Valid {
		description = client.Description.String
	}

	return &ClientResponse{
		ID:           client.ID,
		ClientID:     client.ClientID,
		Name:         client.Name,
		Description:  description,
		RedirectURIs: redirectURIs,
		Scopes:       scopes,
		GrantTypes:   grantTypes,
		IsPublic:     client.IsPublic,
		IsActive:     client.IsActive,
		CreatedBy:    client.CreatedBy,
		CreatedAt:    client.CreatedAt,
		UpdatedAt:    client.UpdatedAt,
	}
}
