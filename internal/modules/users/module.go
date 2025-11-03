package users

import (
	"gogin/internal/clients"
	"gogin/internal/config"
	"gogin/internal/middleware"
	"gogin/internal/modules/redishelper"
	"gogin/internal/utils"

	"github.com/gin-gonic/gin"
)

// UsersModule handles user management
type UsersModule struct {
	service     *UserService
	authMiddleware *middleware.AuthMiddleware
}

// NewUsersModule creates a new users module
func NewUsersModule(db *clients.Database, redis *clients.RedisClient, cfg *config.Config) *UsersModule {
	jwtUtil := utils.NewJWTUtil(cfg.OAuth.JWTSecret, cfg.OAuth.JWTIssuer)
	redisHelper := redishelper.NewRedisHelper(redis)
	authMiddleware := middleware.NewAuthMiddleware(jwtUtil, redisHelper)

	service := NewUserService(db, jwtUtil, redisHelper, cfg)

	return &UsersModule{
		service:     service,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers user routes
func (m *UsersModule) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		// Public routes
		users.POST("/register", m.register)
		users.POST("/login", m.login)

		// Protected routes
		auth := users.Group("")
		auth.Use(m.authMiddleware.RequireAuth())
		{
			auth.GET("/me", m.getProfile)
			auth.PUT("/me", m.updateProfile)
			auth.PUT("/me/password", m.changePassword)
			auth.POST("/logout", m.logout)
			auth.DELETE("/me", m.deleteAccount)
		}

		// Admin routes
		admin := users.Group("")
		admin.Use(m.authMiddleware.RequireAuth())
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("", m.listUsers)
			admin.GET("/:id", m.getUserByID)
			admin.PUT("/:id", m.updateUser)
			admin.DELETE("/:id", m.adminDeleteUser)
			admin.PUT("/:id/status", m.updateUserStatus)
		}
	}
}
