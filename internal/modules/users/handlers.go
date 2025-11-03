package users

import (
	"net/http"
	"strconv"

	"gogin/internal/response"

	"github.com/gin-gonic/gin"
)

// register handles user registration
// @Summary Register a new user
// @Description Create a new user account with email and password
// @Tags Users
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "User registration details"
// @Success 201 {object} response.Response{data=object{user=UserResponse}}
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Failure 400 {object} response.Response
// @Router /users/register [post]
func (m *UsersModule) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	user, err := m.service.CreateUser(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "User registered successfully", gin.H{
		"user": m.service.sanitizeUser(user),
	})
}

// login handles user login
// @Summary User login
// @Description Authenticate user and receive access and refresh tokens
// @Tags Users
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} response.Response{data=LoginResponse}
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Failure 401 {object} response.Response
// @Router /users/login [post]
func (m *UsersModule) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	loginResp, err := m.service.AuthenticateUser(req.Email, req.Password)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Login successful", loginResp)
}

// getProfile retrieves the current user's profile
// @Summary Get user profile
// @Description Get the authenticated user's profile information
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=object{user=UserResponse}}
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /users/me [get]
func (m *UsersModule) getProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := m.service.GetUserByID(userID.(string))
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	response.Success(c, http.StatusOK, "Profile retrieved successfully", gin.H{
		"user": m.service.sanitizeUser(user),
	})
}

// updateProfile updates the current user's profile
// @Summary Update user profile
// @Description Update the authenticated user's profile information
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateProfileRequest true "Profile update details"
// @Success 200 {object} response.Response{data=object{user=UserResponse}}
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Failure 400 {object} response.Response
// @Router /users/me [put]
func (m *UsersModule) updateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	user, err := m.service.UpdateUser(userID.(string), &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Profile updated successfully", gin.H{
		"user": m.service.sanitizeUser(user),
	})
}

// changePassword changes the current user's password
// @Summary Change password
// @Description Change the authenticated user's password
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "Password change details"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Failure 400 {object} response.Response
// @Router /users/me/password [put]
func (m *UsersModule) changePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	err := m.service.ChangePassword(userID.(string), req.OldPassword, req.NewPassword)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", nil)
}

// logout handles user logout
// @Summary User logout
// @Description Logout the authenticated user and invalidate their session
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /users/logout [post]
func (m *UsersModule) logout(c *gin.Context) {
	_, exists := c.Get("token_id")
	if !exists {
		response.Unauthorized(c, "Invalid token")
		return
	}

	userID, _ := c.Get("user_id")

	// Revoke current token
	// Note: We would need to get expiry from token claims to properly revoke
	// For now, we'll delete the session
	if userID != nil {
		m.service.redisHelper.DeleteAllUserSessions(userID.(string))
	}

	response.Success(c, http.StatusOK, "Logged out successfully", nil)
}

// deleteAccount handles account deletion
// @Summary Delete account
// @Description Delete the authenticated user's account
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /users/me [delete]
func (m *UsersModule) deleteAccount(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	err := m.service.DeleteUser(userID.(string))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Account deleted successfully", nil)
}

// Admin handlers

// listUsers lists all users (admin only)
// @Summary List all users
// @Description Get a paginated list of all users (admin only)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} response.Response{data=object{users=[]UserResponse,total=int,page=int,limit=int,total_pages=int}}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users [get]
func (m *UsersModule) listUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	users, total, err := m.service.ListUsers(page, limit)
	if err != nil {
		response.InternalError(c, "Failed to list users")
		return
	}

	// Convert to response format
	userResponses := make([]*UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = m.service.sanitizeUser(user)
	}

	totalPages := (total + limit - 1) / limit

	response.Success(c, http.StatusOK, "Users retrieved successfully", gin.H{
		"users":       userResponses,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// getUserByID retrieves a user by ID (admin only)
// @Summary Get user by ID
// @Description Get a specific user by their ID (admin only)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} response.Response{data=object{user=UserResponse}}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /users/{id} [get]
func (m *UsersModule) getUserByID(c *gin.Context) {
	userID := c.Param("id")

	user, err := m.service.GetUserByID(userID)
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	response.Success(c, http.StatusOK, "User retrieved successfully", gin.H{
		"user": m.service.sanitizeUser(user),
	})
}

// updateUser updates a user (admin only)
// @Summary Update user
// @Description Update a user's profile information (admin only)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param request body UpdateProfileRequest true "Profile update details"
// @Success 200 {object} response.Response{data=object{user=UserResponse}}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Failure 400 {object} response.Response
// @Router /users/{id} [put]
func (m *UsersModule) updateUser(c *gin.Context) {
	userID := c.Param("id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	user, err := m.service.UpdateUser(userID, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User updated successfully", gin.H{
		"user": m.service.sanitizeUser(user),
	})
}

// adminDeleteUser deletes a user (admin only)
// @Summary Delete user
// @Description Delete a user account (admin only)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /users/{id} [delete]
func (m *UsersModule) adminDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	err := m.service.DeleteUser(userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User deleted successfully", nil)
}

// updateUserStatus updates a user's status (admin only)
// @Summary Update user status
// @Description Update a user's status (active, inactive, or suspended) (admin only)
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param request body object{status=string} true "Status update details (status must be: active, inactive, or suspended)"
// @Success 200 {object} response.Response{data=object{status=string}}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 422 {object} response.Response{errors=[]response.ResponseError}
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users/{id}/status [put]
func (m *UsersModule) updateUserStatus(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required,oneof=active inactive suspended"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errors := []response.ResponseError{
			response.NewError("VALIDATION_ERROR", err.Error(), ""),
		}
		response.ValidationError(c, errors)
		return
	}

	// Update user status in database
	query := `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	result, err := m.service.db.Exec(query, req.Status, userID)
	if err != nil {
		response.InternalError(c, "Failed to update user status")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		response.NotFound(c, "User not found")
		return
	}

	// If suspended, revoke all sessions
	if req.Status == "suspended" || req.Status == "inactive" {
		m.service.redisHelper.DeleteAllUserSessions(userID)
	}

	response.Success(c, http.StatusOK, "User status updated successfully", gin.H{
		"status": req.Status,
	})
}
