package models

import (
	"database/sql"
	"time"
)

// User represents a user in the system
type User struct {
	ID            string         `json:"id" db:"id"`
	Email         string         `json:"email" db:"email"`
	PasswordHash  string         `json:"-" db:"password_hash"`
	FirstName     string         `json:"first_name" db:"first_name"`
	LastName      string         `json:"last_name" db:"last_name"`
	Phone         sql.NullString `json:"phone,omitempty" db:"phone"`
	Avatar        sql.NullString `json:"avatar,omitempty" db:"avatar"`
	Role          string         `json:"role" db:"role"` // admin, user, etc.
	Status        string         `json:"status" db:"status"` // active, inactive, suspended
	EmailVerified bool           `json:"email_verified" db:"email_verified"`
	PhoneVerified bool           `json:"phone_verified" db:"phone_verified"`
	LastLoginAt   sql.NullTime   `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" db:"updated_at"`
	DeletedAt     sql.NullTime   `json:"deleted_at,omitempty" db:"deleted_at"`
}

// UserProfile represents additional user profile information
type UserProfile struct {
	UserID      string         `json:"user_id" db:"user_id"`
	Bio         sql.NullString `json:"bio,omitempty" db:"bio"`
	DateOfBirth sql.NullTime   `json:"date_of_birth,omitempty" db:"date_of_birth"`
	Gender      sql.NullString `json:"gender,omitempty" db:"gender"`
	Address     sql.NullString `json:"address,omitempty" db:"address"`
	City        sql.NullString `json:"city,omitempty" db:"city"`
	State       sql.NullString `json:"state,omitempty" db:"state"`
	Country     sql.NullString `json:"country,omitempty" db:"country"`
	ZipCode     sql.NullString `json:"zip_code,omitempty" db:"zip_code"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// FullName returns the user's full name
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// IsActive returns true if user is active
func (u *User) IsActive() bool {
	return u.Status == "active" && !u.DeletedAt.Valid
}

// IsAdmin returns true if user is an admin
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}
