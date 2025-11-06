package notifications

import (
	"encoding/json"
	"fmt"
	"time"

	"gogin/internal/clients"
	"gogin/internal/models"
	"gogin/internal/modules/sendgrid"
	"gogin/internal/modules/twilio"

	"github.com/google/uuid"
)

// NotificationsService handles notifications business logic
type NotificationsService struct {
	db       *clients.Database
	nats     *clients.NATSClient
	sendgrid *sendgrid.SendGridClient
	twilio   *twilio.TwilioClient
}

// NewNotificationsService creates a new notifications service
func NewNotificationsService(db *clients.Database, nats *clients.NATSClient, sg *sendgrid.SendGridClient, tw *twilio.TwilioClient) *NotificationsService {
	return &NotificationsService{
		db:       db,
		nats:     nats,
		sendgrid: sg,
		twilio:   tw,
	}
}

// SendNotification creates and queues a notification
func (s *NotificationsService) SendNotification(req *SendNotificationRequest) (*NotificationResponse, error) {
	id := uuid.New().String()
	query := `
		INSERT INTO notifications (id, user_id, type, channel, title, content, is_read, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := s.db.QueryRow(query,
		id,
		req.UserID,
		req.Type,
		req.Channel,
		req.Title,
		req.Content,
		false,
		"pending",
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	// Queue for async delivery
	notifData, _ := json.Marshal(req)
	go s.nats.Publish("notification.send", notifData)

	return &NotificationResponse{
		ID:        id,
		UserID:    req.UserID,
		Type:      req.Type,
		Channel:   req.Channel,
		Title:     req.Title,
		Content:   req.Content,
		IsRead:    false,
		Status:    "pending",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// ListNotifications lists user notifications
func (s *NotificationsService) ListNotifications(userID string, page, limit int) ([]*NotificationResponse, int, int, error) {
	offset := (page - 1) * limit

	// Get total count
	var total, unread int
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_read = FALSE THEN 1 ELSE 0 END), 0)
		FROM notifications
		WHERE user_id = $1
	`, userID).Scan(&total, &unread)
	if err != nil {
		return nil, 0, 0, err
	}

	// Get notifications
	query := `
		SELECT id, user_id, type, channel, title, content, is_read, read_at, status, created_at, updated_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var notifications []*NotificationResponse
	for rows.Next() {
		var notif models.Notification
		err := rows.Scan(
			&notif.ID,
			&notif.UserID,
			&notif.Type,
			&notif.Channel,
			&notif.Title,
			&notif.Content,
			&notif.IsRead,
			&notif.ReadAt,
			&notif.Status,
			&notif.CreatedAt,
			&notif.UpdatedAt,
		)
		if err != nil {
			return nil, 0, 0, err
		}
		notifications = append(notifications, s.toNotificationResponse(&notif))
	}

	return notifications, total, unread, nil
}

// GetNotification retrieves a notification by ID
func (s *NotificationsService) GetNotification(id, userID string) (*NotificationResponse, error) {
	var notif models.Notification
	query := `
		SELECT id, user_id, type, channel, title, content, is_read, read_at, status, created_at, updated_at
		FROM notifications
		WHERE id = $1 AND user_id = $2
	`

	err := s.db.QueryRow(query, id, userID).Scan(
		&notif.ID,
		&notif.UserID,
		&notif.Type,
		&notif.Channel,
		&notif.Title,
		&notif.Content,
		&notif.IsRead,
		&notif.ReadAt,
		&notif.Status,
		&notif.CreatedAt,
		&notif.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return s.toNotificationResponse(&notif), nil
}

// MarkAsRead marks a notification as read
func (s *NotificationsService) MarkAsRead(id, userID string) error {
	query := `UPDATE notifications SET is_read = TRUE, read_at = NOW(), updated_at = NOW() WHERE id = $1 AND user_id = $2`
	result, err := s.db.Exec(query, id, userID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// DeleteNotification deletes a notification
func (s *NotificationsService) DeleteNotification(id, userID string) error {
	query := `DELETE FROM notifications WHERE id = $1 AND user_id = $2`
	result, err := s.db.Exec(query, id, userID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// SendEmail sends an email via SendGrid
func (s *NotificationsService) SendEmail(to []string, subject, body string) error {
	msg := &sendgrid.EmailMessage{
		To:          to,
		Subject:     subject,
		TextContent: body,
		HTMLContent: fmt.Sprintf("<p>%s</p>", body),
	}
	return s.sendgrid.SendEmail(msg)
}

// SendSMS sends an SMS via Twilio
func (s *NotificationsService) SendSMS(to, body string) error {
	msg := &twilio.SMSMessage{
		To:   to,
		Body: body,
	}
	return s.twilio.SendSMS(msg)
}

// Helper functions

func (s *NotificationsService) toNotificationResponse(notif *models.Notification) *NotificationResponse {
	resp := &NotificationResponse{
		ID:        notif.ID,
		UserID:    notif.UserID,
		Type:      notif.Type,
		Channel:   notif.Channel,
		Title:     notif.Title,
		Content:   notif.Content,
		IsRead:    notif.IsRead,
		Status:    notif.Status,
		CreatedAt: notif.CreatedAt,
		UpdatedAt: notif.UpdatedAt,
	}

	if notif.ReadAt.Valid {
		readAt := notif.ReadAt.Time
		resp.ReadAt = &readAt
	}

	return resp
}
