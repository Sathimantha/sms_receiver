package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// DB is the global database connection
var db *sql.DB

// SMSMessage represents the structure of an SMS message in the database
type SMSMessage struct {
	MessageSID  string
	FromNumber  string
	Body        string
	ReceivedAt  time.Time
}

// logError logs errors in a structured format
func logError(errorType, message string) {
	log.Printf("[%s] %s", errorType, message)
}

// handleSMS handles incoming SMS webhooks from Twilio
func handleSMS(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logError("WEBHOOK_INVALID_FORM", fmt.Sprintf("Failed to parse form data: %v", err))
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Extract parameters from direct form fields (case-insensitive)
	messageSID := r.PostFormValue("MessageSid")
	if messageSID == "" {
		messageSID = r.PostFormValue("messagesid")
	}
	fromNumber := r.PostFormValue("From")
	if fromNumber == "" {
		fromNumber = r.PostFormValue("from")
	}
	body := r.PostFormValue("Body")
	if body == "" {
		body = r.PostFormValue("body")
	}

	// Fallback: Check if 'body' parameter contains URL-encoded parameters
	if messageSID == "" || fromNumber == "" || body == "" {
		bodyParam := r.PostFormValue("body")
		if bodyParam != "" {
			// Remove leading '?' if present
			bodyParam = strings.TrimPrefix(bodyParam, "?")
			// Decode URL-encoded body
			parsed, err := url.ParseQuery(bodyParam)
			if err != nil {
				logError("WEBHOOK_INVALID_BODY", fmt.Sprintf("Failed to parse body parameter: %v", err))
				http.Error(w, "Invalid body parameter", http.StatusBadRequest)
				return
			}
			if messageSID == "" {
				messageSID = parsed.Get("MessageSid")
				if messageSID == "" {
					messageSID = parsed.Get("messagesid")
				}
			}
			if fromNumber == "" {
				fromNumber = parsed.Get("From")
				if fromNumber == "" {
					fromNumber = parsed.Get("from")
				}
			}
			if body == "" {
				body = parsed.Get("Body")
				if body == "" {
					body = parsed.Get("body")
				}
			}
		}
	}

	// Validate required fields
	if messageSID == "" || fromNumber == "" || body == "" {
		logError("WEBHOOK_NO_INPUT", fmt.Sprintf("Missing required fields: MessageSid=%s, From=%s, Body=%s", messageSID, fromNumber, body))
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Create SMS message struct
	sms := SMSMessage{
		MessageSID:  messageSID,
		FromNumber:  fromNumber,
		Body:        body,
		ReceivedAt:  time.Now(),
	}

	// Save to database
	err := saveSMS(sms)
	if err != nil {
		logError("DB_SAVE_ERROR", fmt.Sprintf("Failed to save SMS to database: %v", err))
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	log.Printf("Saved SMS from %s: %s", fromNumber, body)

	// Respond with TwiML to acknowledge webhook
	twimlResponse := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>Message received! Thank you.</Message>
</Response>`

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(twimlResponse))
	if err != nil {
		logError("WEBHOOK_RESPONSE_ERROR", fmt.Sprintf("Error writing TwiML response: %v", err))
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
		logError("STARTUP_ERROR", fmt.Sprintf("Error loading .env file: %v", err))
		os.Exit(1)
	}

	// Database and server configuration
	dbUser := os.Getenv("DB_USERNAME")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	listenPort := os.Getenv("LISTEN_PORT")
	certFile := os.Getenv("CERT_FILE")
	keyFile := os.Getenv("KEY_FILE")

	// Validate environment variables
	if dbUser == "" || dbPass == "" || dbHost == "" || dbPort == "" || dbName == "" || listenPort == "" || certFile == "" || keyFile == "" {
		logError("CONFIG_ERROR", "Missing required environment variables")
		os.Exit(1)
	}

	// MySQL connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbHost, dbPort, dbName)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		logError("DB_CONNECTION_ERROR", fmt.Sprintf("Failed to connect to DB: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	// Test database connection
	err = db.Ping()
	if err != nil {
		logError("DB_PING_ERROR", fmt.Sprintf("Database ping failed: %v", err))
		os.Exit(1)
	}

	// Verify certificate and key files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		logError("CONFIG_ERROR", fmt.Sprintf("Certificate file not found: %s", certFile))
		os.Exit(1)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		logError("CONFIG_ERROR", fmt.Sprintf("Key file not found: %s", keyFile))
		os.Exit(1)
	}

	// Initialize router
	r := mux.NewRouter()
	r.HandleFunc("/sms", handleSMS).Methods("POST")

	// Enable CORS and logging
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	// Start HTTPS server
	log.Printf("Starting HTTPS server on port %s", listenPort)
	if err := http.ListenAndServeTLS(":"+listenPort, certFile, keyFile, loggedRouter); err != nil {
		logError("SERVER_ERROR", fmt.Sprintf("Failed to start HTTPS server: %v", err))
		os.Exit(1)
	}
}