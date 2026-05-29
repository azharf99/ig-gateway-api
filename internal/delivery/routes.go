package delivery

import (
	"net/http"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/delivery/http/handlers"
	"github.com/azharf99/ig-gateway-api/internal/delivery/http/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter(authH *handlers.AuthHandler, postH *handlers.PostHandler) *gin.Engine {
	r := gin.Default()

	// Apply CORS
	r.Use(middleware.CORSMiddleware())

	// Serve static files (Crucial: Instagram API fetches media files from here)
	r.Static("/uploads", "./uploads")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK", "timestamp": time.Now().Format(time.RFC3339)})
	})

	api := r.Group("/api/v1")
	{
		// Auth endpoints
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authH.Register)
			authGroup.POST("/login", authH.Login)
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
			postsGroup.GET("", postH.GetPosts) // Use postH.GetPosts directly
			postsGroup.GET("/:id", postH.GetPostByID)
			postsGroup.DELETE("/:id", postH.DeletePost)
		}
	}

	return r
}
