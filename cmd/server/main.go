package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"CampusMoon/internals/handlers"
	"CampusMoon/internals/storage"
)

func main() {
	// Initialize database and MinIO
	storage.InitDB()
	storage.InitMinIO()

	// Setup SMTP Config from environment variables
	handlers.SMTPConfig = handlers.SMTPConfiguration{
		Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		Port:     getEnv("SMTP_PORT", "587"),
		Username: getEnv("SMTP_USERNAME", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", ""),
	}

	// Setup routes
	http.HandleFunc("/email", handlers.ServeEmail)
	http.HandleFunc("/send-email", handlers.SendEmailHandler)
	http.HandleFunc("/meet", handlers.ServeMeet)
	http.HandleFunc("/upload", handlers.UploadHandler)
	http.HandleFunc("/videos", handlers.VideosHandler)
	http.HandleFunc("/delete", handlers.DeleteVideoHandler)
	http.HandleFunc("/watch", handlers.VideoPageHandler)

	// WebRTC + Chat
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.Handlewebrtc(w, r, storage.DB)
	})
	http.HandleFunc("/chat/history", func(w http.ResponseWriter, r *http.Request) {
		handlers.ChatHistory(w, r, storage.DB)
	})

	// Admin & Auth
	http.HandleFunc("/welcome", handlers.ServeWelcome)
	http.HandleFunc("/admin", handlers.ServeAdmin)
	http.HandleFunc("/admin/register", handlers.AdminAPI)
	http.HandleFunc("/student", handlers.ServeStudent)
	http.HandleFunc("/staff", handlers.ServeStaff)
	http.HandleFunc("/login", handlers.LoginHandler)
	http.HandleFunc("/labs", handlers.ServeLabs)

	// Poll
	http.HandleFunc("/polls", handlers.ServePoll)
	http.HandleFunc("/ws_poll", handlers.HandleConnectionsPoll)
	go handlers.HandleMessagesPoll()

	// Code runner
	http.HandleFunc("/run", handlers.RunHandler)
	http.HandleFunc("/code", handlers.ServeCodeRunner)
	http.HandleFunc("/cs", handlers.Cs)
	http.HandleFunc("/elabs", handlers.Serveelabs)

	http.HandleFunc("/", handlers.ServeHome)

	//questions for videos
    http.HandleFunc("/exam", handlers.ServeExam)
	http.HandleFunc("/api/generate-exam", handlers.GenerateExamHandler)

	// Start server
	port := getEnv("PORT", "8080")
	fmt.Printf("🚀 Server running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}


func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}