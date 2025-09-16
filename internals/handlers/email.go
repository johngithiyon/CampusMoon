package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/smtp"
	"strings"
)

// EmailRequest for JSON API
type EmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// EmailResponse returned to frontend
type EmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SMTPConfiguration stores SMTP credentials
type SMTPConfiguration struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// Global variable to hold config set in main.go
var SMTPConfig SMTPConfiguration

// ServeIndex renders the index.html page
func ServeEmail(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/email.html")
}

// SendEmailHandler handles POST request to /send-email
func SendEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var emailReq EmailRequest
	err := json.NewDecoder(r.Body).Decode(&emailReq)
	if err != nil {
		sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	if emailReq.To == "" || emailReq.Subject == "" || emailReq.Body == "" || !isValidEmail(emailReq.To) {
		sendErrorResponse(w, "Missing or invalid fields", http.StatusBadRequest)
		return
	}

	err = sendEmail(emailReq.To, emailReq.Subject, emailReq.Body)
	if err != nil {
		log.Println("Error sending email:", err)
		sendErrorResponse(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(EmailResponse{Success: true, Message: "Email sent successfully"})
}

// sendEmail uses smtp package to send the mail
func sendEmail(to, subject, body string) error {
	from := SMTPConfig.From
	if from == "" {
		from = SMTPConfig.Username
	}

	auth := smtp.PlainAuth("", SMTPConfig.Username, SMTPConfig.Password, SMTPConfig.Host)

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" + body

	return smtp.SendMail(SMTPConfig.Host+":"+SMTPConfig.Port, auth, from, []string{to}, []byte(msg))
}

func isValidEmail(email string) bool {
	at := strings.Index(email, "@")
	dot := strings.LastIndex(email, ".")
	return len(email) >= 3 && at > 0 && dot > at+1 && dot < len(email)-1
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(EmailResponse{Success: false, Message: message})
}
