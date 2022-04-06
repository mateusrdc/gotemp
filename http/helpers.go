package http

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func parseExpiration(expiration string) (time.Time, error) {
	if expiration != "" {
		if expiration == "never" {
			return time.Time{}, nil
		} else {
			// Try to parse ISO-8601 datestring
			parsed_time, err := time.Parse(time.RFC3339, expiration)

			if err != nil {
				return time.Time{}, errors.New("invalid expiration date, use the RFC-3339 format")
			}

			// Dont allow a time in the past
			if parsed_time.Before(time.Now()) {
				return time.Time{}, errors.New("provide a time in the future")
			}

			return parsed_time, nil
		}
	} else {
		// No expiration time set, default to 24 hours from now
		return time.Now().Add(time.Hour * 24), nil
	}
}

func validateJwt(attempt_token string) bool {
	token, _ := jwt.Parse(attempt_token, func(t *jwt.Token) (interface{}, error) {
		return secret_key, nil
	})

	return token.Valid
}

func validateJwtFromRequest(c echo.Context) bool {
	header := c.Request().Header.Get("Authorization")

	// Valid tokens are in the following format:
	// bearer secretkeyhere
	if header == "" || header[6:7] != " " {
		return false
	}

	// Make sure the token is valid
	return validateJwt(header[7:])
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)

	if errors.Is(err, os.ErrNotExist) || !info.IsDir() {
		return false
	}

	return true
}
