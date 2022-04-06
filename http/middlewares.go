package http

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

func AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Make sure the tokens matches
			if !validateJwtFromRequest(c) {
				return c.JSON(http.StatusUnauthorized, echo.Map{"success": false, "error": "Unauthorized"})
			}

			return next(c)
		}
	}
}

func CorsMiddleware() echo.MiddlewareFunc {
	allow_origins := GetEnv("CORS_ALLOWED_ORIGINS", "")
	allow_methods := GetEnv("CORS_ALLOWED_METHODS", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	allow_headers := GetEnv("CORS_ALLOWED_HEADERS", "Origin, Authorization, Content-Type")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", allow_origins)

			if allow_headers != "" {
				c.Response().Header().Set("Access-Control-Allow-Headers", allow_headers)
			}

			if allow_methods != "" {
				c.Response().Header().Set("Access-Control-Allow-Methods", allow_methods)
			}

			if c.Request().Method == "OPTIONS" {
				c.Response().Status = 204
				return nil
			}

			return next(c)
		}
	}
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
