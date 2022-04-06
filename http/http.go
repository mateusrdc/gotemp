package http

import (
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

var secret_key []byte

func Init(db *gorm.DB, key []byte) {
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

	initAPI(e, db)

	e.GET("/socket", socketHandler)

	if GetEnv("HTTP_DISABLE_WEBUI", "false") != "true" {
		if isDirectory("public") {
			e.Static("/", "public")
		} else {
			log.Println("WebUI disabled due to absent 'public' folder.")
		}
	}

	log.Println("Starting HTTP server at", GetEnv("HTTP_ADDRESS", ":2527"))
	e.Start(GetEnv("HTTP_ADDRESS", ":2527"))
}
