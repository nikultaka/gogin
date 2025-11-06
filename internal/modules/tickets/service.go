package tickets

import (
	"database/sql"
	"fmt"
	"time"

	"gogin/internal/clients"
	"gogin/internal/config"
	"gogin/internal/models"
	"gogin/internal/modules/redishelper"
)

type TicketsService struct {
	db          *clients.Database
	redisHelper *redishelper.RedisHelper
	config      *config.Config
}

func NewTicketsService(db *clients.Database, redisHelper *redishelper.RedisHelper, cfg *config.Config) *TicketsService {
	return &TicketsService{
		db:          db,
		redisHelper: redisHelper,
		config:      cfg,
	}
}

// toTicketResponse converts a models.SupportTicket to TicketResponse
func (s *TicketsService) toTicketResponse(ticket *models.SupportTicket) *TicketResponse {
	response := &TicketResponse{
		ID:          ticket.ID,
		UserID:      ticket.UserID,
		Subject:     ticket.Subject,
		Description: ticket.Description,
		Status:      ticket.Status,
		Priority:    ticket.Priority,
		CreatedAt:   ticket.CreatedAt,
		UpdatedAt:   ticket.UpdatedAt,
	}

	if ticket.Category.Valid {
		category := ticket.Category.String
		response.Category = &category
	}

	if ticket.AssignedTo.Valid {
		assignedTo := ticket.AssignedTo.String
		response.AssignedTo = &assignedTo
	}

	if ticket.ResolvedAt.Valid {
		response.ResolvedAt = &ticket.ResolvedAt.Time
	}

	if ticket.ClosedAt.Valid {
		response.ClosedAt = &ticket.ClosedAt.Time
	}

	return response
}

// toReplyResponse converts a models.SupportTicketReply to ReplyResponse
func (s *TicketsService) toReplyResponse(reply *models.SupportTicketReply) *ReplyResponse {
	response := &ReplyResponse{
		ID:        reply.ID,
		TicketID:  reply.TicketID,
		UserID:    reply.UserID,
		IsStaff:   reply.IsStaff,
		Content:   reply.Content,
		CreatedAt: reply.CreatedAt,
		UpdatedAt: reply.UpdatedAt,
	}

	if reply.DeletedAt.Valid {
		response.DeletedAt = &reply.DeletedAt.Time
	}

	return response
}

// CreateTicket creates a new support ticket
func (s *TicketsService) CreateTicket(userID string, req *CreateTicketRequest) (*TicketResponse, error) {
	query := `
		INSERT INTO support_tickets (user_id, subject, description, priority, category, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at
	`

	now := time.Now().UTC()
	var ticket models.SupportTicket

	category := sql.NullString{String: req.Category, Valid: req.Category != ""}

	err := s.db.QueryRow(
		query,
		userID,
		req.Subject,
		req.Description,
		req.Priority,
		category,
		"open",
		now,
		now,
	).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Subject,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&ticket.Category,
		&ticket.AssignedTo,
		&ticket.ResolvedAt,
		&ticket.ClosedAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Invalidate user tickets cache
	s.redisHelper.CacheDelete(fmt.Sprintf("user_tickets:%s", userID))

	return s.toTicketResponse(&ticket), nil
}

// GetTicketByID retrieves a ticket by ID
func (s *TicketsService) GetTicketByID(ticketID string) (*TicketResponse, error) {
	query := `
		SELECT id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at
		FROM support_tickets
		WHERE id = $1
	`

	var ticket models.SupportTicket
	err := s.db.QueryRow(query, ticketID).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Subject,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&ticket.Category,
		&ticket.AssignedTo,
		&ticket.ResolvedAt,
		&ticket.ClosedAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	return s.toTicketResponse(&ticket), nil
}

// GetTicketWithReplies retrieves a ticket with all its replies
func (s *TicketsService) GetTicketWithReplies(ticketID string) (*TicketDetailResponse, error) {
	// Get ticket
	ticket, err := s.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}

	// Get replies
	query := `
		SELECT id, ticket_id, user_id, is_staff, content, created_at, updated_at, deleted_at
		FROM support_ticket_replies
		WHERE ticket_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := s.db.Query(query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get replies: %w", err)
	}
	defer rows.Close()

	var replies []*ReplyResponse
	for rows.Next() {
		var reply models.SupportTicketReply
		if err := rows.Scan(
			&reply.ID,
			&reply.TicketID,
			&reply.UserID,
			&reply.IsStaff,
			&reply.Content,
			&reply.CreatedAt,
			&reply.UpdatedAt,
			&reply.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reply: %w", err)
		}
		replies = append(replies, s.toReplyResponse(&reply))
	}

	if replies == nil {
		replies = []*ReplyResponse{}
	}

	return &TicketDetailResponse{
		Ticket:  ticket,
		Replies: replies,
	}, nil
}

// ListUserTickets lists all tickets for a specific user
func (s *TicketsService) ListUserTickets(userID string, status string, page, limit int) (*TicketsListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Build query
	countQuery := `SELECT COUNT(*) FROM support_tickets WHERE user_id = $1`
	query := `
		SELECT id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at
		FROM support_tickets
		WHERE user_id = $1
	`

	args := []interface{}{userID}

	if status != "" {
		countQuery += ` AND status = $2`
		query += ` AND status = $2`
		args = append(args, status)
	}

	// Count total
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count tickets: %w", err)
	}

	// Query tickets
	query += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*TicketResponse
	for rows.Next() {
		var ticket models.SupportTicket
		if err := rows.Scan(
			&ticket.ID,
			&ticket.UserID,
			&ticket.Subject,
			&ticket.Description,
			&ticket.Status,
			&ticket.Priority,
			&ticket.Category,
			&ticket.AssignedTo,
			&ticket.ResolvedAt,
			&ticket.ClosedAt,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}
		tickets = append(tickets, s.toTicketResponse(&ticket))
	}

	if tickets == nil {
		tickets = []*TicketResponse{}
	}

	totalPages := (total + limit - 1) / limit

	return &TicketsListResponse{
		Tickets:    tickets,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// ListAllTickets lists all tickets (admin only)
func (s *TicketsService) ListAllTickets(status, priority string, page, limit int) (*TicketsListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Build query
	countQuery := `SELECT COUNT(*) FROM support_tickets WHERE 1=1`
	query := `
		SELECT id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at
		FROM support_tickets
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 0

	if status != "" {
		argCount++
		countQuery += fmt.Sprintf(` AND status = $%d`, argCount)
		query += fmt.Sprintf(` AND status = $%d`, argCount)
		args = append(args, status)
	}

	if priority != "" {
		argCount++
		countQuery += fmt.Sprintf(` AND priority = $%d`, argCount)
		query += fmt.Sprintf(` AND priority = $%d`, argCount)
		args = append(args, priority)
	}

	// Count total
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count tickets: %w", err)
	}

	// Query tickets
	argCount++
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argCount, argCount+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*TicketResponse
	for rows.Next() {
		var ticket models.SupportTicket
		if err := rows.Scan(
			&ticket.ID,
			&ticket.UserID,
			&ticket.Subject,
			&ticket.Description,
			&ticket.Status,
			&ticket.Priority,
			&ticket.Category,
			&ticket.AssignedTo,
			&ticket.ResolvedAt,
			&ticket.ClosedAt,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}
		tickets = append(tickets, s.toTicketResponse(&ticket))
	}

	if tickets == nil {
		tickets = []*TicketResponse{}
	}

	totalPages := (total + limit - 1) / limit

	return &TicketsListResponse{
		Tickets:    tickets,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// UpdateTicket updates a ticket
func (s *TicketsService) UpdateTicket(ticketID, userID string, req *UpdateTicketRequest) (*TicketResponse, error) {
	// Build dynamic update query
	query := `UPDATE support_tickets SET updated_at = $1`
	args := []interface{}{time.Now().UTC()}
	argCount := 1

	if req.Subject != "" {
		argCount++
		query += fmt.Sprintf(`, subject = $%d`, argCount)
		args = append(args, req.Subject)
	}

	if req.Description != "" {
		argCount++
		query += fmt.Sprintf(`, description = $%d`, argCount)
		args = append(args, req.Description)
	}

	if req.Priority != "" {
		argCount++
		query += fmt.Sprintf(`, priority = $%d`, argCount)
		args = append(args, req.Priority)
	}

	if req.Category != "" {
		argCount++
		query += fmt.Sprintf(`, category = $%d`, argCount)
		args = append(args, req.Category)
	}

	argCount++
	query += fmt.Sprintf(` WHERE id = $%d AND user_id = $%d`, argCount, argCount+1)
	query += ` RETURNING id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at`
	args = append(args, ticketID, userID)

	var ticket models.SupportTicket
	err := s.db.QueryRow(query, args...).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Subject,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&ticket.Category,
		&ticket.AssignedTo,
		&ticket.ResolvedAt,
		&ticket.ClosedAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found or access denied")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Invalidate cache
	s.redisHelper.CacheDelete(fmt.Sprintf("user_tickets:%s", userID))

	return s.toTicketResponse(&ticket), nil
}

// UpdateTicketStatus updates the status of a ticket (admin only)
func (s *TicketsService) UpdateTicketStatus(ticketID string, req *UpdateTicketStatusRequest) (*TicketResponse, error) {
	now := time.Now().UTC()
	var resolvedAt, closedAt sql.NullTime

	// Set timestamps based on status
	if req.Status == "resolved" {
		resolvedAt = sql.NullTime{Time: now, Valid: true}
	} else if req.Status == "closed" {
		closedAt = sql.NullTime{Time: now, Valid: true}
	}

	query := `
		UPDATE support_tickets
		SET status = $1, resolved_at = $2, closed_at = $3, updated_at = $4
		WHERE id = $5
		RETURNING id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at
	`

	var ticket models.SupportTicket
	err := s.db.QueryRow(query, req.Status, resolvedAt, closedAt, now, ticketID).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Subject,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&ticket.Category,
		&ticket.AssignedTo,
		&ticket.ResolvedAt,
		&ticket.ClosedAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update ticket status: %w", err)
	}

	// Invalidate cache
	s.redisHelper.CacheDelete(fmt.Sprintf("user_tickets:%s", ticket.UserID))

	return s.toTicketResponse(&ticket), nil
}

// AssignTicket assigns a ticket to an admin (admin only)
func (s *TicketsService) AssignTicket(ticketID string, req *AssignTicketRequest) (*TicketResponse, error) {
	query := `
		UPDATE support_tickets
		SET assigned_to = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, user_id, subject, description, status, priority, category, assigned_to, resolved_at, closed_at, created_at, updated_at
	`

	now := time.Now().UTC()
	var ticket models.SupportTicket

	err := s.db.QueryRow(query, req.AssignedTo, now, ticketID).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Subject,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&ticket.Category,
		&ticket.AssignedTo,
		&ticket.ResolvedAt,
		&ticket.ClosedAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to assign ticket: %w", err)
	}

	return s.toTicketResponse(&ticket), nil
}

// CreateReply creates a reply to a ticket
func (s *TicketsService) CreateReply(ticketID, userID string, isStaff bool, req *CreateReplyRequest) (*ReplyResponse, error) {
	query := `
		INSERT INTO support_ticket_replies (ticket_id, user_id, is_staff, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, ticket_id, user_id, is_staff, content, created_at, updated_at, deleted_at
	`

	now := time.Now().UTC()
	var reply models.SupportTicketReply

	err := s.db.QueryRow(query, ticketID, userID, isStaff, req.Content, now, now).Scan(
		&reply.ID,
		&reply.TicketID,
		&reply.UserID,
		&reply.IsStaff,
		&reply.Content,
		&reply.CreatedAt,
		&reply.UpdatedAt,
		&reply.DeletedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create reply: %w", err)
	}

	return s.toReplyResponse(&reply), nil
}

// DeleteTicket deletes a ticket (user can only delete their own open tickets)
func (s *TicketsService) DeleteTicket(ticketID, userID string) error {
	query := `DELETE FROM support_tickets WHERE id = $1 AND user_id = $2 AND status = 'open'`

	result, err := s.db.Exec(query, ticketID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete ticket: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("ticket not found or cannot be deleted")
	}

	// Invalidate cache
	s.redisHelper.CacheDelete(fmt.Sprintf("user_tickets:%s", userID))

	return nil
}
