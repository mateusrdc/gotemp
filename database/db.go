package database

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type MailBox struct {
	ID          string    `gorm:"type:varchar(36)" json:"id"`
	Name        string    `json:"name"`
	Address     string    `gorm:"unique" json:"address"`
	Emails      []Mail    `gorm:"constraint:OnDelete:CASCADE;" json:"emails"`
	Locked      bool      `json:"locked"`
	UnreadCount uint      `json:"unread_count"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	LastEmailAt time.Time `json:"last_email_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Mail struct {
	ID        string    `gorm:"type:varchar(36)" json:"id"`
	Subject   string    `json:"subject"`
	From      string    `json:"from"`
	To        string    `gorm:"index" json:"to"`
	Body      string    `json:"body"`
	Headers   string    `json:"headers"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	MailBoxID string    `json:"-"`
}

func (mb *MailBox) BeforeCreate(tx *gorm.DB) (err error) {
	uuid, err := uuid.NewRandom()

	if err != nil {
		err = errors.New("couldn't  generate uuid")
	}

	mb.ID = uuid.String()

	return
}

func (m *Mail) BeforeCreate(tx *gorm.DB) (err error) {
	uuid, err := uuid.NewRandom()

	if err != nil {
		err = errors.New("couldn't  generate uuid")
	}

	m.ID = uuid.String()

	return
}

func Init() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("data.db"), &gorm.Config{})

	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&MailBox{}, &Mail{})

	return db, nil
}
