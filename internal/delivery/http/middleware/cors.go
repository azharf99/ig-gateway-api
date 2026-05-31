package middleware

import (
	"net/http"
	"strings"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	var allowedOriginsMap map[string]bool
	if config.AppConfig != nil && config.AppConfig.AllowedOrigins != "" {
		allowedOrigins := strings.Split(config.AppConfig.AllowedOrigins, ",")
		allowedOriginsMap = make(map[string]bool)
		for _, origin := range allowedOrigins {
			allowedOriginsMap[strings.TrimSpace(origin)] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" && allowedOriginsMap != nil && allowedOriginsMap[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

