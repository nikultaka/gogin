package models

import (
	"database/sql"
	"time"
)

// Notification represents a notification record
type Notification struct {
	ID          string         `json:"id" db:"id"`
	UserID      string         `json:"user_id" db:"user_id"`
	Type        string         `json:"type" db:"type"`
	Channel     string         `json:"channel" db:"channel"` // email, sms, push
	Title       string         `json:"title" db:"title"`
	Content     string         `json:"content" db:"content"`
	IsRead      bool           `json:"is_read" db:"is_read"`
	ReadAt      sql.NullTime   `json:"read_at,omitempty" db:"read_at"`
	Status      string         `json:"status" db:"status"` // pending, sent, failed
	Recipient   sql.NullString `json:"recipient,omitempty" db:"recipient"`
	Subject     sql.NullString `json:"subject,omitempty" db:"subject"`
	Provider    sql.NullString `json:"provider,omitempty" db:"provider"`
	ProviderID  sql.NullString `json:"provider_id,omitempty" db:"provider_id"`
	ErrorMsg    sql.NullString `json:"error_msg,omitempty" db:"error_msg"`
	Attempts    int            `json:"attempts" db:"attempts"`
	SentAt      sql.NullTime   `json:"sent_at,omitempty" db:"sent_at"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}
