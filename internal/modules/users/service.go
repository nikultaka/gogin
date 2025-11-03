package users

import (
	"database/sql"
	"fmt"
	"time"

	"gogin/internal/clients"
	"gogin/internal/config"
	"gogin/internal/models"
	"gogin/internal/modules/redishelper"
	"gogin/internal/utils"

	"github.com/google/uuid"
)

// UserService handles user business logic
type UserService struct {
	db          *clients.Database
	jwtUtil     *utils.JWTUtil
	redisHelper *redishelper.RedisHelper
	config      *config.Config
}

// NewUserService creates a new user service
func NewUserService(db *clients.Database, jwtUtil *utils.JWTUtil, redisHelper *redishelper.RedisHelper, cfg *config.Config) *UserService {
	return &UserService{
		db:          db,
		jwtUtil:     jwtUtil,
		redisHelper: redisHelper,
		config:      cfg,
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(req *RegisterRequest) (*models.User, error) {
	// Validate email
	if !utils.IsEmailValid(req.Email) {
		return nil, fmt.Errorf("invalid email address")
	}

	// Validate password
	valid, msg := utils.IsPasswordValid(req.Password)
	if !valid {
		return nil, fmt.Errorf(msg)
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Check if email already exists
	exists, err := s.emailExists(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("email already registered")
	}

	// Create user
	user := &models.User{
		ID:            uuid.New().String(),
		Email:         utils.SanitizeString(req.Email),
		PasswordHash:  hashedPassword,
		FirstName:     utils.SanitizeString(req.FirstName),
		LastName:      utils.SanitizeString(req.LastName),
		Role:          "user",
		Status:        "active",
		EmailVerified: false,
		PhoneVerified: false,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, status, email_verified, phone_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, email, first_name, last_name, role, status, email_verified, phone_verified, created_at, updated_at
	`

	err = s.db.QueryRow(
		query,
		user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName,
		user.Role, user.Status, user.EmailVerified, user.PhoneVerified, user.CreatedAt, user.UpdatedAt,
	).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName,
		&user.Role, &user.Status, &user.EmailVerified, &user.PhoneVerified, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create user profile
	if err := s.createUserProfile(user.ID); err != nil {
		return nil, fmt.Errorf("failed to create user profile: %w", err)
	}

	return user, nil
}

// AuthenticateUser authenticates a user and returns tokens
func (s *UserService) AuthenticateUser(email, password string) (*LoginResponse, error) {
	// Get user by email
	user, err := s.getUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive() {
		return nil, fmt.Errorf("account is inactive or deleted")
	}

	// Verify password
	if !utils.VerifyPassword(password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate tokens
	accessToken, _, err := s.jwtUtil.GenerateAccessToken(
		user.ID,
		"web", // default client
		user.Role,
		[]string{"read", "write"},
		s.config.OAuth.AccessTokenExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, refreshTokenID, err := s.jwtUtil.GenerateRefreshToken(
		user.ID,
		"web",
		s.config.OAuth.RefreshTokenExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Update last login
	s.updateLastLogin(user.ID)

	// Store refresh token
	s.storeRefreshToken(user.ID, refreshTokenID, s.config.OAuth.RefreshTokenExpiry)

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.config.OAuth.AccessTokenExpiry.Seconds()),
		User:         s.sanitizeUser(user),
	}, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(userID string) (*models.User, error) {
	query := `
		SELECT id, email, first_name, last_name, phone, avatar, role, status,
		       email_verified, phone_verified, last_login_at, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	user := &models.User{}
	err := s.db.QueryRow(query, userID).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Avatar,
		&user.Role, &user.Status, &user.EmailVerified, &user.PhoneVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(userID string, req *UpdateProfileRequest) (*models.User, error) {
	query := `
		UPDATE users
		SET first_name = $1, last_name = $2, phone = $3, updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING id, email, first_name, last_name, phone, avatar, role, status,
		          email_verified, phone_verified, last_login_at, created_at, updated_at
	`

	user := &models.User{}
	err := s.db.QueryRow(
		query,
		req.FirstName, req.LastName, req.Phone, time.Now().UTC(), userID,
	).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Avatar,
		&user.Role, &user.Status, &user.EmailVerified, &user.PhoneVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Invalidate user cache
	s.redisHelper.CacheDelete(fmt.Sprintf("user:%s", userID))

	return user, nil
}

// ChangePassword changes user password
func (s *UserService) ChangePassword(userID, oldPassword, newPassword string) error {
	// Get user
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Verify old password
	if !utils.VerifyPassword(oldPassword, user.PasswordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	// Validate new password
	valid, msg := utils.IsPasswordValid(newPassword)
	if !valid {
		return fmt.Errorf(msg)
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	query := `UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`
	_, err = s.db.Exec(query, hashedPassword, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all existing sessions
	s.redisHelper.DeleteAllUserSessions(userID)

	return nil
}

// DeleteUser soft deletes a user
func (s *UserService) DeleteUser(userID string) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	result, err := s.db.Exec(query, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	// Revoke all sessions
	s.redisHelper.DeleteAllUserSessions(userID)

	// Invalidate cache
	s.redisHelper.CacheDelete(fmt.Sprintf("user:%s", userID))

	return nil
}

// ListUsers lists all users with pagination
func (s *UserService) ListUsers(page, limit int) ([]*models.User, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	err := s.db.QueryRow(countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Get users
	query := `
		SELECT id, email, first_name, last_name, phone, avatar, role, status,
		       email_verified, phone_verified, last_login_at, created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Avatar,
			&user.Role, &user.Status, &user.EmailVerified, &user.PhoneVerified,
			&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, total, nil
}

// Helper methods

func (s *UserService) emailExists(email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`
	err := s.db.QueryRow(query, email).Scan(&exists)
	return exists, err
}

func (s *UserService) getUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, phone, avatar, role, status,
		       email_verified, phone_verified, last_login_at, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	user := &models.User{}
	err := s.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName,
		&user.Phone, &user.Avatar, &user.Role, &user.Status, &user.EmailVerified,
		&user.PhoneVerified, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *UserService) createUserProfile(userID string) error {
	query := `INSERT INTO user_profiles (user_id, created_at, updated_at) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, userID, time.Now().UTC(), time.Now().UTC())
	return err
}

func (s *UserService) updateLastLogin(userID string) {
	query := `UPDATE users SET last_login_at = $1 WHERE id = $2`
	s.db.Exec(query, time.Now().UTC(), userID)
}

func (s *UserService) storeRefreshToken(userID, tokenID string, expiry time.Duration) {
	key := fmt.Sprintf("refresh_token:%s", tokenID)
	s.redisHelper.CacheSet(key, map[string]string{"user_id": userID}, expiry)
}

func (s *UserService) sanitizeUser(user *models.User) *UserResponse {
	return &UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Phone:         user.Phone.String,
		Avatar:        user.Avatar.String,
		Role:          user.Role,
		Status:        user.Status,
		EmailVerified: user.EmailVerified,
		PhoneVerified: user.PhoneVerified,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}
