package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"gotemp/api"
	"gotemp/database"
	"gotemp/smtp"

	"github.com/dchest/uniuri"
)

var db *gorm.DB
var secret_key string

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
	generate_new_key := flag.Bool("generate-key", false, "Whether to generate a new secret key")

	flag.Parse()

	if *generate_new_key {
		new_key := uniuri.NewLen(128)

		os.WriteFile("key.secret", []byte(new_key), 0700)
		secret_key = new_key

		log.Fatalf("Done! Your new secret key is: %s\n", new_key)
	} else {
		if data, err := os.ReadFile("key.secret"); err != nil {
			log.Fatalln("ERROR: Couldn't load key.secret file, Run the program with --generate-key to generate a new key.")
		} else {
			secret_key = string(data)

			if len(secret_key) != 128 {
				log.Println("ERROR: Your secret key appears to be invalid (length isn't 128).")
				log.Fatalln("ERROR: Please fix it or create a new key with the --generate-key parameter.")
			}
		}
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
	go api.Init(db, secret_key)

	// Init auto deleter
	go database.InitAutoCleaner(db)

	// Waint until ctrl c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
