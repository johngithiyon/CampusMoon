package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"CampusMoon/internals/handlers"
	"CampusMoon/internals/storage"
)

func main() {
	// Initialize DB and MinIO via your storage package
	storage.InitDB()   // assume this sets storage.DB or similar
	storage.InitMinIO()

	// If your storage package exposes storage.DB, use it. Otherwise call storage.GetDB() or similar.
	var db *sql.DB
	// attempt to use storage.DB if present (older code used storage.DB)
	// adapt this to how your storage package exposes the DB:
	db = storage.DB
	if db == nil {
		// if storage.DB isn't set, try a getter or fail
		log.Println("warning: storage.DB is nil; discussion DB will be disabled")
	}

	// Setup SMTP config (unchanged)
	handlers.SMTPConfig = handlers.SMTPConfiguration{
		Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		Port:     getEnv("SMTP_PORT", "587"),
		Username: getEnv("SMTP_USERNAME", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", ""),
	}

	// Register your app routes (existing)
	http.HandleFunc("/email", handlers.ServeEmail)
	http.HandleFunc("/send-email", handlers.SendEmailHandler)

	http.HandleFunc("/meet", handlers.ServeMeet)
	http.HandleFunc("/upload", handlers.UploadHandler)
	http.HandleFunc("/videos", handlers.VideosHandler)
	http.HandleFunc("/delete", handlers.DeleteVideoHandler)
	http.HandleFunc("/watch", handlers.VideoPageHandler)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.Handlewebrtc(w, r, db)
	})
	http.HandleFunc("/chat/history", func(w http.ResponseWriter, r *http.Request) {
		handlers.ChatHistory(w, r, db)
	})

	// --- Discussion routes ---
	handlers.InitDiscussion(db)
	http.HandleFunc("/discussion", handlers.ServeDiscussionPage)    // serves templates/discussion.html
	http.HandleFunc("/ws_discussion", handlers.HandleDiscussionWS)  // web socket (text + binary audio)
	http.HandleFunc("/discussion/history", handlers.ChatHistoryHandler_discussion)

	// Admin & Auth etc (keep your existing registrations)
	http.HandleFunc("/welcome", handlers.ServeWelcome)
	http.HandleFunc("/admin", handlers.ServeAdmin)
	http.HandleFunc("/admin/register", handlers.AdminAPI)
	http.HandleFunc("/student", handlers.ServeStudent)
	http.HandleFunc("/staff", handlers.ServeStaff)
	http.HandleFunc("/login", handlers.LoginHandler)
	http.HandleFunc("/labs", handlers.ServeLabs)

	// Polls
	http.HandleFunc("/polls", handlers.ServePoll)
	http.HandleFunc("/ws_poll", handlers.HandleConnectionsPoll)
	go handlers.HandleMessagesPoll()

	// Code runner
	http.HandleFunc("/run", handlers.RunHandler)
	http.HandleFunc("/code", handlers.ServeCodeRunner)
	http.HandleFunc("/cs", handlers.Cs)
	http.HandleFunc("/elabs", handlers.Serveelabs)

	http.HandleFunc("/", handlers.ServeHome)

	// Exam
	http.HandleFunc("/exam", handlers.ServeExam)
	http.HandleFunc("/api/generate-exam", handlers.GenerateExamHandler)

	// Start
	port := getEnv("PORT", "8080")
	fmt.Printf("🚀 Server running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
