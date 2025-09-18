package main

import (
	"CampusMoon/internals/handlers"
	"CampusMoon/internals/storage"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize DB and MinIO
	storage.InitDB()
	storage.InitMinIO()

	db := storage.DB
	if db == nil {
		log.Println("warning: storage.DB is nil; some features may not work")
	}

	// ---------------- Audio ----------------
	handlers.InitAudio()
	// ---------------------------------------

	// Setup SMTP config
	handlers.SMTPConfig = handlers.SMTPConfiguration{
		Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		Port:     getEnv("SMTP_PORT", "587"),
		Username: getEnv("SMTP_USERNAME", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", ""),
	}

	// Create router
	r := mux.NewRouter()

	// ---------------- Audio Routes ----------------
	handlers.RegisterAudioRoutes(r)

	// ---------------- Email Routes ----------------
	r.HandleFunc("/email", handlers.ServeEmail)
	r.HandleFunc("/send-email", handlers.SendEmailHandler)

	// ---------------- Video Routes ----------------
	r.HandleFunc("/meet", handlers.ServeMeet)
	r.HandleFunc("/upload", handlers.UploadHandler).Methods("POST")
	r.HandleFunc("/videos", handlers.VideosHandler)
	r.HandleFunc("/delete", handlers.DeleteVideoHandler)
	r.HandleFunc("/watch", handlers.VideoPageHandler)

	// ---------------- WebRTC & Chat ----------------
	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.Handlewebrtc(w, r, db)
	})
	r.HandleFunc("/chat/history", func(w http.ResponseWriter, r *http.Request) {
		handlers.ChatHistory(w, r, db)
	})

	// ---------------- Discussion ----------------
	handlers.InitDiscussion(db)
	r.HandleFunc("/discussion", handlers.ServeDiscussionPage)
	r.HandleFunc("/ws_discussion", handlers.HandleDiscussionWS)
	r.HandleFunc("/discussion/history", handlers.ChatHistoryHandler_discussion)

	// ---------------- Admin & Auth ----------------
	r.HandleFunc("/welcome", handlers.ServeWelcome)
	r.HandleFunc("/admin", handlers.ServeAdmin)
	r.HandleFunc("/admin/register", handlers.AdminAPI)
	r.HandleFunc("/student", handlers.ServeStudent)
	r.HandleFunc("/staff", handlers.ServeStaff)
	r.HandleFunc("/login", handlers.LoginHandler)
	r.HandleFunc("/labs", handlers.ServeLabs)

	// ---------------- Polls ----------------
	r.HandleFunc("/polls", handlers.ServePoll)
	r.HandleFunc("/ws_poll", handlers.HandleConnectionsPoll)
	go handlers.HandleMessagesPoll()

	// ---------------- Code Runner ----------------
	r.HandleFunc("/run", handlers.RunHandler)
	r.HandleFunc("/code", handlers.ServeCodeRunner)
	r.HandleFunc("/cs", handlers.Cs)
	r.HandleFunc("/elabs", handlers.Serveelabs)

	// ---------------- Exam ----------------
	r.HandleFunc("/exam", handlers.ServeExam)
	r.HandleFunc("/api/generate-exam", handlers.GenerateExamHandler)

	// ---------------- Home Page ----------------
	r.HandleFunc("/", handlers.ServeHome)

	// ---------------- Converter ----------------
	// Serve uploads folder
	os.MkdirAll(handlers.UploadDir, os.ModePerm)
	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(handlers.UploadDir))))

	// Convert to text route
	r.HandleFunc("/convert-to-text", handlers.ConvertToTextHandler).Methods("POST", "OPTIONS")

	// Start server
	port := getEnv("PORT", "8080")
	fmt.Printf("🚀 Server running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, r))
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
