package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var db *sql.DB

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		os.Exit(1)
	}

	// Database connection setup
	dbUser := os.Getenv("DB_USERNAME")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	listenPort := os.Getenv("LISTEN_PORT")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		os.Exit(1)
	}
	defer db.Close()

	// Initialize router
	r := mux.NewRouter()

	// Define route for incoming SMS
	r.HandleFunc("/sms", smsHandler).Methods("POST")

	// Apply CORS to allow all origins
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"POST"}),
		handlers.AllowedHeaders([]string{"Content-Type"}),
	)

	// Wrap router with CORS
	http.Handle("/", corsHandler(r))

	// Start server with TLS
	certFile := os.Getenv("CERT_FILE")
	keyFile := os.Getenv("KEY_FILE")
	if certFile == "" || keyFile == "" || listenPort == "" {
		os.Exit(1)
	}

	err = http.ListenAndServeTLS(":"+listenPort, certFile, keyFile, nil)
	if err != nil {
		os.Exit(1)
	}
}

func smsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Extract Twilio form fields
	from := r.PostFormValue("From")
	body := r.PostFormValue("Body")
	messageSid := r.PostFormValue("MessageSid")

	// Validate inputs
	if from == "" || body == "" || messageSid == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Basic input sanitization (limit length to prevent abuse)
	if len(body) > 1600 || len(from) > 15 || len(messageSid) > 50 {
		http.Error(w, "Input length exceeded", http.StatusBadRequest)
		return
	}

	// Store SMS in database
	query := `INSERT INTO sms_messages (message_sid, from_number, body, received_at) VALUES (?, ?, ?, ?)`
	timestamp := time.Now().UTC()
	_, err := db.Exec(query, messageSid, from, body, timestamp)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Respond with simple success message
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Message stored successfully"))
}
