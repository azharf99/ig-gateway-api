package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/gin-gonic/gin"
)

func RateLimiterMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Redis == nil {
			c.Next()
			return
		}

		identifier := c.ClientIP()
		if userID, exists := c.Get("userID"); exists {
			identifier = fmt.Sprintf("user:%v", userID)
		}

		key := fmt.Sprintf("rate_limit:%s", identifier)
		ctx := context.Background()

		count, err := config.Redis.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		if count == 1 {
			config.Redis.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Please try again later."})
			c.Abort()
			return
		}

		c.Next()
	}
}
