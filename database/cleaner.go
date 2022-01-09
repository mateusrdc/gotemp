package database

import (
	"time"

	"gorm.io/gorm"
)

func InitAutoCleaner(db *gorm.DB) {
	go func() {
		for {
			db.Exec("DELETE FROM mail_boxes WHERE expires_at <> \"0001-01-01 00:00:00+00:00\" AND ? > expires_at", time.Now())

			time.Sleep(time.Hour * 1)
		}
	}()
}
