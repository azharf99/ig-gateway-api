package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/gin-gonic/gin"
)

type rateLimiterItem struct {
	count     int
	expiresAt time.Time
}

var (
	memoryStore = make(map[string]*rateLimiterItem)
	storeMutex  sync.Mutex
)

func init() {
	// Background routine to clean up expired memory rate limit data to prevent memory leak
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			storeMutex.Lock()
			now := time.Now()
			for k, v := range memoryStore {
				if now.After(v.expiresAt) {
					delete(memoryStore, k)
				}
			}
			storeMutex.Unlock()
		}
	}()
}

func memoryRateLimit(key string, limit int, window time.Duration) bool {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	now := time.Now()
	item, exists := memoryStore[key]
	if !exists || now.After(item.expiresAt) {
		memoryStore[key] = &rateLimiterItem{
			count:     1,
			expiresAt: now.Add(window),
		}
		return true
	}

	item.count++
	if item.count > limit {
		return false
	}
	return true
}

func RateLimiterMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		identifier := c.ClientIP()
		if userID, exists := c.Get("userID"); exists {
			identifier = fmt.Sprintf("user:%v", userID)
		}

		key := fmt.Sprintf("rate_limit:%s", identifier)

		if config.Redis == nil {
			if !memoryRateLimit(key, limit, window) {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Please try again later."})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		ctx := context.Background()
		count, err := config.Redis.Incr(ctx, key).Result()
		if err != nil {
			// Fail-secure: fallback to in-memory rate limiting when Redis fails
			if !memoryRateLimit(key, limit, window) {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Please try again later."})
				c.Abort()
				return
			}
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

