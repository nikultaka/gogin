package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"gogin/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuditLogger middleware logs API requests to audit_logs table
type AuditLogger struct {
	db *clients.Database
}

// NewAuditLogger creates a new audit logger middleware
func NewAuditLogger(db *clients.Database) *AuditLogger {
	return &AuditLogger{db: db}
}

// Log returns middleware that logs requests to audit log
func (a *AuditLogger) Log() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for health/status endpoints
		if c.Request.URL.Path == "/api/v1/health" || c.Request.URL.Path == "/api/v1/status" {
			c.Next()
			return
		}

		// Record start time
		startTime := time.Now()

		// Get request body
		var requestBody string
		if c.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			requestBody = string(bodyBytes)
		}

		// Process request
		c.Next()

		// Get user info from context
		userID := ""
		if uid, exists := c.Get("user_id"); exists {
			userID = uid.(string)
		}

		clientID := ""
		if cid, exists := c.Get("client_id"); exists {
			clientID = cid.(string)
		}

		// Prepare metadata
		metadata := map[string]interface{}{
			"method":         c.Request.Method,
			"path":           c.Request.URL.Path,
			"query":          c.Request.URL.RawQuery,
			"ip":             c.ClientIP(),
			"user_agent":     c.Request.UserAgent(),
			"status_code":    c.Writer.Status(),
			"duration_ms":    time.Since(startTime).Milliseconds(),
			"request_id":     c.GetString("request_id"),
		}

		metadataJSON, _ := json.Marshal(metadata)

		// Insert audit log asynchronously
		go a.insertAuditLog(
			userID,
			clientID,
			c.Request.Method+" "+c.Request.URL.Path,
			requestBody,
			string(metadataJSON),
			c.ClientIP(),
		)
	}
}

func (a *AuditLogger) insertAuditLog(userID, clientID, action, requestData, metadata, ipAddress string) {
	// Parse action to extract resource (e.g., "GET /api/v1/users" -> resource: "/api/v1/users")
	resource := action
	status := "success"

	query := `
		INSERT INTO audit_logs (id, user_id, client_id, action, resource, ip_address, user_agent, metadata, status, created_at)
		VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, ''), $4, $5, $6, $7, $8::jsonb, $9, NOW())
	`

	_, err := a.db.Exec(query,
		uuid.New().String(),
		userID,
		clientID,
		action,
		resource,
		ipAddress,
		"", // user_agent is already in metadata
		metadata,
		status,
	)

	if err != nil {
		// Log error but don't fail the request
		println("Failed to insert audit log:", err.Error())
	}
}
