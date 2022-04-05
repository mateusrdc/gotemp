package api

import (
	"gotemp/database"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

type API struct {
	Database   *gorm.DB
	ServerName string
}

type MailBoxForm struct {
	Name       string `json:"name" form:"name" binding:"required,min=1,max=64"`
	Address    string `json:"address" form:"address" binding:"required,min=1,max=64,excludes=@"`
	Locked     bool   `json:"locked" form:"locked"`
	Expiration string `json:"expires_at" form:"expires_at"`
}

var secret_key string

func Init(db *gorm.DB, key string) {
	api := API{Database: db, ServerName: GetEnv("SMTP_DOMAIN", "gotemp")}

	secret_key = key

	e := echo.New()

	if GetEnv("DEBUG", "false") == "true" {
		e.Debug = true
		e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
			Format:           "${time_custom} [API REQUEST] | ${status} | ${latency_human} | ${remote_ip} | ${method} ${uri}\n",
			CustomTimeFormat: "2006/01/02 15:04:05",
		}))
	}

	e.HideBanner = true
	e.HidePort = true
	e.Use(CorsMiddleware())

	g := e.Group("")
	{
		g.Use(AuthMiddleware(key))

		g.GET("/status", api.GetStatus)

		g.GET("/mailboxes", api.GetAll)
		g.GET("/mailboxes/:id", api.GetOne)
		g.PUT("/mailboxes/:id", api.EditOne)
		g.POST("/mailboxes", api.Create)
		g.DELETE("/mailboxes/:id", api.Delete)
		g.DELETE("/mailboxes/:id/mails", api.DeleteEmails)
		g.PUT("/mailboxes/:id/:mailid/read", api.MarkEmailRead)
	}

	e.GET("/socket", socketHandler)

	log.Println("Starting API server at", GetEnv("API_ADDRESS", ":2527"))
	e.Start(GetEnv("API_ADDRESS", ":2527"))
}

// GET /status: checks whether the client is authorized to make further queries
// {success: bool}
func (api *API) GetStatus(c echo.Context) error {
	return c.JSON(200, echo.Map{"success": true, "server_name": api.ServerName})
}

// GET /mailboxes: returns all mailboxes
// {success: bool, mailboxes: []MailBox}
func (api *API) GetAll(c echo.Context) error {
	var mailboxes []database.MailBox

	api.Database.Order("last_email_at desc").Find(&mailboxes)

	return c.JSON(200, echo.Map{"success": true, "mailboxes": mailboxes})
}

// GET /mailboxes/:id: gets an specific mailbox contents (including emails)
// {success: bool, mailbox: MailBox}
func (api *API) GetOne(c echo.Context) error {
	var mailbox database.MailBox

	if q := api.Database.Where("id = ?", c.Param("id")).Preload("Emails").Limit(1).Find(&mailbox); q.RowsAffected == 0 {
		return c.JSON(400, echo.Map{"success": false, "error": "Invalid mailbox"})
	}

	return c.JSON(200, echo.Map{"success": true, "mailbox": mailbox})
}

// PUT /mailboxes/:id/ - edit mailbox (locked, title etc)
// {success: bool, id: string}
func (api *API) EditOne(c echo.Context) error {
	var input MailBoxForm

	if e := c.Bind(&input); e != nil {
		return c.JSON(400, echo.Map{"success": false, "error": e.Error()})
	}

	// Check if the email exists
	var mailbox database.MailBox

	if q := api.Database.Where("id = ?", c.Param("id")).Limit(1).Find(&mailbox); q.RowsAffected == 0 {
		return c.JSON(400, echo.Map{"success": false, "error": "Invalid mailbox"})
	}

	// Update its properties
	mailbox.Name = input.Name
	mailbox.Address = input.Address
	mailbox.Locked = input.Locked

	// Try to parse expiration time (if set)
	time, err := parseExpiration(input.Expiration)

	if err != nil {
		return c.JSON(400, echo.Map{"success": false, "error": err.Error()})
	}

	mailbox.ExpiresAt = time

	// Save model
	api.Database.Save(&mailbox)

	SendSocketMessage("MAILBOX_EDITED", mailbox)
	return c.JSON(200, echo.Map{"success": true, "id": c.Param("id")})
}

// POST /mailboxes: creates a new mailbox
// {success: bool, id: string}
func (api *API) Create(c echo.Context) error {
	var data MailBoxForm

	if e := c.Bind(&data); e != nil {
		return c.JSON(400, echo.Map{"success": false, "error": e.Error()})
	}

	model := database.MailBox{
		Name:    data.Name,
		Address: data.Address,
		Locked:  data.Locked,
	}

	// Try to parse expiration time (if set)
	time, err := parseExpiration(data.Expiration)

	if err != nil {
		return c.JSON(400, echo.Map{"success": false, "error": err.Error()})
	}

	model.ExpiresAt = time

	// Make sure a mailbox with the choosen address doesn't exists already
	if q := api.Database.Where("address = ?", data.Address).Limit(1).Find(&database.MailBox{}); q.RowsAffected != 0 {
		return c.JSON(400, echo.Map{"success": false, "error": "A Mailbox with this address already exists!"})
	}

	if q := api.Database.Create(&model); q.RowsAffected == 0 {
		return c.JSON(500, echo.Map{"success": false, "error": q.Error.Error()})
	}

	SendSocketMessage("MAILBOX_CREATED", model)
	return c.JSON(200, echo.Map{"success": true, "id": model.ID})
}

// DELETE /mailboxes/:id: deletes a mailbox
// {success: bool, id: string}
func (api *API) Delete(c echo.Context) error {
	if q := api.Database.Where("id = ?", c.Param("id")).Delete(&database.MailBox{}); q.RowsAffected == 0 {
		return c.JSON(400, echo.Map{"success": false, "error": "Invalid mailbox"})
	}

	// Delete emails associated with this mailbox
	api.Database.Exec("DELETE FROM mails WHERE mail_box_id = ?", c.Param("id"))

	SendSocketMessage("MAILBOX_DELETED", c.Param("id"))
	return c.JSON(200, echo.Map{"success": true, "id": c.Param("id")})
}

// DELETE /mailboxes/:id/emails deletes email(s)
// {success: bool, id: string}
func (api *API) DeleteEmails(c echo.Context) error {
	var input []string

	if e := c.Bind(&input); e != nil {
		return c.JSON(400, echo.Map{"success": false, "error": e.Error()})
	}

	if len(input) > 50 {
		return c.JSON(400, echo.Map{"success": false, "error": "Too many emails to delete"})
	}

	// Update unread count
	query := `UPDATE mail_boxes
			  SET unread_count = unread_count - (SELECT count(*) FROM mails WHERE mail_box_id = ? AND read = 0 AND id IN ?)
			  WHERE id = ?`

	api.Database.Exec(query, c.Param("id"), input, c.Param("id"))

	// Delete items from database
	api.Database.Exec("DELETE FROM mails WHERE mail_box_id = ? AND id IN ?", c.Param("id"), input)

	return c.JSON(200, echo.Map{"success": true})
}

// PUT /mailboxes/:id/:mailid/read - mark a email as read
// {success: bool, id: string}
func (api *API) MarkEmailRead(c echo.Context) error {
	if q := api.Database.Model(&database.Mail{}).Where("id = ? AND mail_box_id = ? AND read = ?", c.Param("mailid"), c.Param("id"), false).Update("read", true); q.RowsAffected == 0 {
		return c.JSON(400, echo.Map{"success": false, "error": "Invalid email and/or mailbox"})
	}

	api.Database.Exec("UPDATE mail_boxes SET unread_count = unread_count - 1 WHERE id = ?", c.Param("id"))

	return c.JSON(200, echo.Map{"success": true, "id": c.Param("mailid")})
}
