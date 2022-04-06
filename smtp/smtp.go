package smtp

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"gotemp/database"
	"gotemp/http"

	"github.com/emersion/go-smtp"
	"gorm.io/gorm"
)

// var db *gorm.DB
// var debug = Getenv("DEBUG", "false") == "true"
var (
	db            *gorm.DB
	debug         bool
	server_domain string
)

// The Backend implements SMTP server methods.
type Backend struct{}

func (bkd *Backend) NewSession(_ smtp.ConnectionState, _ string) (smtp.Session, error) {
	return &Session{}, nil
}

// A Session is returned after EHLO.
type Session struct {
	from    string
	to      string
	mailbox *database.MailBox
}

func (s *Session) AuthPlain(username, password string) error {
	return errors.New("invalid username or password")
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	DebugPrintln("Mail from:", from)

	s.from = from

	return nil
}

func (s *Session) Rcpt(to string) error {
	DebugPrintln("Rcpt to:", to)

	// Check if the 'to' is valid
	if !strings.HasSuffix(to, "@"+server_domain) {
		log.Println("Invalid mailbox (1): " + to)
		return errors.New("invalid 'to' address")
	}

	// Check if the target mailbox exists
	var mailbox database.MailBox

	if q := db.Where("address = ?", strings.TrimSuffix(to, "@"+server_domain)).Limit(1).Find(&mailbox); q.RowsAffected == 0 {
		log.Println("Invalid mailbox (2): " + to)
		return errors.New("invalid 'to' address")
	}

	s.to = to
	s.mailbox = &mailbox

	return nil
}

func (s *Session) Data(r io.Reader) error {
	// Just ignore the email if the mailbox is locked
	if s.mailbox.Locked {
		log.Println("Locked mailbox: " + s.to)
		return nil
	}

	// Try to read data
	if b, err := ioutil.ReadAll(r); err != nil {
		return err
	} else {
		data := string(b)

		// Parse email headers (simple)
		headers, err := GetHeaders(data)

		if err != nil {
			DebugPrintln("Error parsing headers")
			return errors.New("error parsing email headers")
		}

		// Parse headers & body to save them later
		body, headers_raw := ParseData(data, false)

		if body == "" {
			// Parse again but this time log out what's happening
			ParseData(data, true)

			return nil
		}

		// Save mail to the database
		model := database.Mail{
			Subject:   decodeMimeHeader(headers.Get("Subject")),
			From:      s.from,
			To:        s.to,
			Body:      body,
			Headers:   headers_raw,
			MailBoxID: s.mailbox.ID,
		}

		db.Create(&model)

		// Update mailbox's last email time
		s.mailbox.LastEmailAt = time.Now()
		s.mailbox.UnreadCount++

		// Send the new email over socket to clients
		http.SendSocketMessage("NEW_EMAIL", map[string]interface{}{"mailbox_id": s.mailbox.ID, "email": model})

		db.Save(s.mailbox)
	}
	return nil
}

func (s *Session) Reset() {
	if debug {
		log.Println("RESET")
	}
}

func (s *Session) Logout() error {
	if debug {
		log.Println("LOGOUT")
	}

	return nil
}

func TestMail(from, to, data string) {
	s := Session{from: from, to: to}
	s.Data(strings.NewReader(data))
}

func Init(db_ *gorm.DB) {
	db = db_
	debug = Getenv("DEBUG", "false") == "true"
	server_domain = Getenv("SMTP_DOMAIN", "localhost")

	be := &Backend{}

	s := smtp.NewServer(be)

	s.Addr = fmt.Sprintf(":%s", Getenv("SMTP_PORT", "25"))
	s.Domain = server_domain
	s.ReadTimeout = 20 * time.Second
	s.WriteTimeout = 20 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 3
	s.AllowInsecureAuth = true

	DebugPrintln("SMTP Debug enabled")
	log.Println("Starting SMTP server at", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func Getenv(key, fallback string) string {
	if env, ok := os.LookupEnv(key); ok {
		return env
	}

	return fallback
}

func DebugPrintln(v ...interface{}) {
	if debug {
		log.Println(v...)
	}
}

func DebugPrintf(format string, v ...interface{}) {
	if debug {
		log.Printf(format, v...)
	}
}
