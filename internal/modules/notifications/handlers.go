package notifications

import (
	"net/http"
	"strconv"

	"gogin/internal/response"

	"github.com/gin-gonic/gin"
)

// listNotifications lists user notifications
// @Summary List Notifications
// @Description Get paginated list of user notifications
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} response.Response{data=NotificationsListResponse}
// @Failure 401 {object} response.Response
// @Router /notifications [get]
func (m *NotificationsModule) listNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	notifications, total, unread, err := m.service.ListNotifications(userID.(string), page, limit)
	if err != nil {
		response.InternalError(c, "Failed to list notifications")
		return
	}

	totalPages := (total + limit - 1) / limit

	response.Success(c, http.StatusOK, "Notifications retrieved successfully", gin.H{
		"notifications": notifications,
		"total":         total,
		"unread":        unread,
		"page":          page,
		"limit":         limit,
		"total_pages":   totalPages,
	})
}

// getNotification retrieves a notification by ID
// @Summary Get Notification
// @Description Get a notification by ID
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response{data=NotificationResponse}
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /notifications/{id} [get]
func (m *NotificationsModule) getNotification(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	notif, err := m.service.GetNotification(id, userID.(string))
	if err != nil {
		response.NotFound(c, "Notification not found")
		return
	}

	response.Success(c, http.StatusOK, "Notification retrieved successfully", notif)
}

// markAsRead marks a notification as read
// @Summary Mark Notification as Read
// @Description Mark a notification as read
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /notifications/{id}/read [put]
func (m *NotificationsModule) markAsRead(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	err := m.service.MarkAsRead(id, userID.(string))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Notification marked as read", nil)
}

// deleteNotification deletes a notification
// @Summary Delete Notification
// @Description Delete a notification
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /notifications/{id} [delete]
func (m *NotificationsModule) deleteNotification(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	err := m.service.DeleteNotification(id, userID.(string))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Notification deleted successfully", nil)
}

// testEmail sends a test email
// @Summary Test Email
// @Description Send a test email via SendGrid
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body TestEmailRequest true "Email details"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Router /notifications/test-email [post]
func (m *NotificationsModule) testEmail(c *gin.Context) {
	var req TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	err := m.service.SendEmail([]string{req.To}, req.Subject, req.Body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Test email sent successfully", nil)
}

// testSMS sends a test SMS
// @Summary Test SMS
// @Description Send a test SMS via Twilio
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body TestSMSRequest true "SMS details"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Router /notifications/test-sms [post]
func (m *NotificationsModule) testSMS(c *gin.Context) {
	var req TestSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	err := m.service.SendSMS(req.To, req.Body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Test SMS sent successfully", nil)
}
