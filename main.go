package main

import (
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"gotemp/database"
	"gotemp/http"
	"gotemp/smtp"
)

var db *gorm.DB
var secret_key []byte

func main() {
	godotenv.Load()

	// Init logging
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0700)

	if err != nil {
		panic(err)
	}

	mw := io.MultiWriter(os.Stdout, logFile)

	log.SetOutput(mw)

	// Try to load secret key from file
	if data, err := os.ReadFile("key.secret"); err != nil {
		log.Println("Couldn't load key.secret file, Use the WebUI to set a key.")
	} else {
		secret_key = data
	}

	// Init database
	db_, err := database.Init()

	if err != nil {
		log.Fatalln("Error connecting to the database", err.Error())
	}

	db_.Exec("PRAGMA foreign_keys = ON")
	db = db_

	// Init SMTP server
	go smtp.Init(db)

	// Init API
	go http.Init(db, secret_key)

	// Init auto deleter
	go database.InitAutoCleaner(db)

	// Waint until ctrl c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
