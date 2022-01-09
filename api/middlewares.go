package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(key string) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		header := c.Request.Header.Get("Authorization")

		// Valid tokens are in the following format:
		// bearer 94083a69h866055ef6x9fe216f968446e133...(128 chars)
		// We can fail fast by checking if the length matches
		if header == "" || len(header) != (128+7) || header[6:7] != " " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
			return
		}

		// Make surethe token matches
		if header[7:] != key {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
			return
		}

		c.Next()
	}

	return fn
}

func CorsMiddleware() gin.HandlerFunc {
	allow_origins := GetEnv("CORS_ALLOWED_ORIGINS", "")
	allow_methods := GetEnv("CORS_ALLOWED_METHODS", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	allow_headers := GetEnv("CORS_ALLOWED_HEADERS", "Origin, Authorization, Content-Type")

	fn := func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", allow_origins)

		if allow_headers != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", allow_headers)
		}

		if allow_methods != "" {
			c.Writer.Header().Set("Access-Control-Allow-Methods", allow_methods)
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}

	return fn
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
