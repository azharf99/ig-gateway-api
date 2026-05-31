package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds secure HTTP headers to prevent common vulnerabilities
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME-sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filtering in browsers
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Enforce HTTPS
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Strict Content Security Policy for APIs
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; sandbox")

		// Referrer policy
		c.Writer.Header().Set("Referrer-Policy", "no-referrer")

		// Feature/Permissions Policy
		c.Writer.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), payment=()")

		c.Next()
	}
}
