package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// DB is the global database connection
var db *sql.DB

// SMSMessage represents the structure of an SMS message in the database
type SMSMessage struct {
	MessageSID string
	FromNumber string
	Body       string
	ReceivedAt time.Time
}

// handleSMS handles incoming SMS webhooks from Twilio
func handleSMS(w http.ResponseWriter, r *http.Request) {
	// Extract Twilio webhook parameters
	messageSID := r.FormValue("MessageSid")
	fromNumber := r.FormValue("From")
	body := r.FormValue("Body")

	if messageSID == "" || fromNumber == "" || body == "" {
		log.Printf("Invalid webhook data: MessageSid=%s, From=%s, Body=%s", messageSID, fromNumber, body)
		http.Error(w, "Invalid webhook data", http.StatusBadRequest)
		return
	}

	// Create SMS message struct
	sms := SMSMessage{
		MessageSID: messageSID,
		FromNumber: fromNumber,
		Body:       body,
		ReceivedAt: time.Now(),
	}

	// Save to database
	err := saveSMS(sms)
	if err != nil {
		log.Printf("Failed to save SMS to database: %v", err)
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	log.Printf("Saved SMS from %s: %s", fromNumber, body)

	// Respond with TwiML to acknowledge webhook
	// Using manual TwiML to avoid twiml.Message dependency issues
	twimlResponse := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>Message received! Thank you.</Message>
</Response>`

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(twimlResponse))
	if err != nil {
		log.Printf("Error writing TwiML response: %v", err)
	}
}

// saveSMS inserts an SMS message into the database
func saveSMS(sms SMSMessage) error {
	query := `
		INSERT INTO sms_messages (message_sid, from_number, body, received_at)
		VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, sms.MessageSID, sms.FromNumber, sms.Body, sms.ReceivedAt)
	return err
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Database connection setup
	dbUser := os.Getenv("DB_USERNAME")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	listenPort := os.Getenv("LISTEN_PORT")

	if dbUser == "" || dbPass == "" || dbHost == "" || dbPort == "" || dbName == "" || listenPort == "" {
		log.Fatal("Missing required environment variables")
	}

	// MySQL configuration
	cfg := mysql.Config{
		User:   dbUser,
		Passwd: dbPass,
		Net:    "tcp",
		Addr:   fmt.Sprintf("%s:%s", dbHost, dbPort),
		DBName: dbName,
	}

	// Initialize database connection
	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	// Initialize router
	r := mux.NewRouter()
	r.HandleFunc("/sms", handleSMS).Methods("POST")

	// Enable CORS and logging
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	// Start server
	log.Printf("Starting server on port %s", listenPort)
	if err := http.ListenAndServe(":"+listenPort, loggedRouter); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
