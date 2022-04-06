package http

import (
	"errors"
	"time"
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
