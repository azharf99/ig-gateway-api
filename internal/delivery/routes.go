package delivery

import (
	"net/http"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/delivery/http/handlers"
	"github.com/azharf99/ig-gateway-api/internal/delivery/http/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter(authH *handlers.AuthHandler, postH *handlers.PostHandler, webhookH *handlers.WebhookHandler) *gin.Engine {
	r := gin.Default()

	// Apply MaxMultipartMemory limit (50MB)
	r.MaxMultipartMemory = 50 << 20

	// Apply global security headers
	r.Use(middleware.SecurityHeadersMiddleware())

	// Apply CORS
	r.Use(middleware.CORSMiddleware())

	// Serve static files (Crucial: Instagram API fetches media files from here)
	r.Static("/uploads", "./uploads")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK", "timestamp": time.Now().Format(time.RFC3339)})
	})

	// Public Webhooks for Meta Data Deletion Compliance
	r.POST("/webhooks/instagram/data-deletion", middleware.RateLimiterMiddleware(5, 1*time.Minute), webhookH.DataDeletionCallback)

	api := r.Group("/api/v1")
	{
		// Auth endpoints
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", middleware.RateLimiterMiddleware(10, 1*time.Minute), authH.Register)
			authGroup.POST("/login", middleware.RateLimiterMiddleware(10, 1*time.Minute), authH.Login)
			authGroup.GET("/me", middleware.AuthMiddleware(), authH.GetMe)
			
			// OAuth URLs and token exchange
			authGroup.GET("/instagram/url", middleware.AuthMiddleware(), authH.GetInstagramOAuthURL)
			authGroup.POST("/instagram/link", middleware.AuthMiddleware(), authH.LinkInstagram)
		}

		// Post endpoints (protected)
		postsGroup := api.Group("/posts")
		postsGroup.Use(middleware.AuthMiddleware())
		postsGroup.Use(middleware.RateLimiterMiddleware(60, 1*time.Minute)) // Rate limit: 60 reqs/min
		{
			postsGroup.POST("", postH.CreatePost)
			postsGroup.GET("", postH.GetPosts)
			postsGroup.GET("/:id", postH.GetPostByID)
			postsGroup.DELETE("/:id", postH.DeletePost)
		}
	}

	return r
}

