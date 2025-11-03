package oauth2

import (
	"gogin/internal/clients"
	"gogin/internal/config"
	"gogin/internal/middleware"
	"gogin/internal/modules/redishelper"
	"gogin/internal/utils"

	"github.com/gin-gonic/gin"
)

// OAuth2Module handles OAuth 2.0 authorization server
type OAuth2Module struct {
	db          *clients.Database
	redis       *clients.RedisClient
	config      *config.Config
	service     *OAuth2Service
	redisHelper *redishelper.RedisHelper
	jwtUtil     *utils.JWTUtil
}

// NewOAuth2Module creates a new OAuth2 module
func NewOAuth2Module(db *clients.Database, redis *clients.RedisClient, cfg *config.Config) *OAuth2Module {
	redisHelper := redishelper.NewRedisHelper(redis)
	jwtUtil := utils.NewJWTUtil(cfg.OAuth.JWTSecret, cfg.OAuth.JWTIssuer)
	service := NewOAuth2Service(db, redisHelper, jwtUtil, cfg)

	return &OAuth2Module{
		db:          db,
		redis:       redis,
		config:      cfg,
		service:     service,
		redisHelper: redisHelper,
		jwtUtil:     jwtUtil,
	}
}

// RegisterRoutes registers OAuth2 routes
func (m *OAuth2Module) RegisterRoutes(router *gin.RouterGroup) {
	oauth := router.Group("/oauth")
	{
		// Public endpoints
		oauth.POST("/authorize", m.authorize)
		oauth.POST("/token", m.token)

		// Protected endpoints
		authMiddleware := middleware.NewAuthMiddleware(m.jwtUtil, m.redisHelper)
		oauth.POST("/revoke", authMiddleware.RequireAuth(), m.revoke)
		oauth.POST("/introspect", authMiddleware.RequireAuth(), m.introspect)
	}
}
