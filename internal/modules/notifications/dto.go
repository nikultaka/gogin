package notifications

import "time"

// NotificationResponse represents a notification response
type NotificationResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Channel   string    `json:"channel"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	IsRead    bool      `json:"is_read"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotificationsListResponse represents a paginated list of notifications
type NotificationsListResponse struct {
	Notifications []*NotificationResponse `json:"notifications"`
	Total         int                     `json:"total"`
	Unread        int                     `json:"unread"`
	Page          int                     `json:"page"`
	Limit         int                     `json:"limit"`
	TotalPages    int                     `json:"total_pages"`
}

// TestEmailRequest represents a test email request
type TestEmailRequest struct {
	To      string `json:"to" binding:"required,email"`
	Subject string `json:"subject" binding:"required"`
	Body    string `json:"body" binding:"required"`
}

// TestSMSRequest represents a test SMS request
type TestSMSRequest struct {
	To   string `json:"to" binding:"required"`
	Body string `json:"body" binding:"required"`
}

// SendNotificationRequest represents a notification send request
type SendNotificationRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	Type    string `json:"type" binding:"required"`
	Channel string `json:"channel" binding:"required,oneof=email sms push"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}
