package main

import (
	"fmt"
	"log"
	"net/http"

	"CampusMoon/internals/storage"
	"CampusMoon/internals/handlers"
)

func main() {
	// Init DB
	storage.InitDB()

	// Init MinIO
	storage.InitMinIO()

	// Routes
	http.HandleFunc("/", handlers.ServeHome)
	http.HandleFunc("/meet", handlers.ServeMeet)
	http.HandleFunc("/upload", handlers.UploadHandler)
	http.HandleFunc("/videos", handlers.VideosHandler)

	// WebRTC + Chat (pass DB)
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
	http.HandleFunc("/labs",handlers.ServeLabs)

	//poll

	http.HandleFunc("/polls", handlers.ServePoll)	
	http.HandleFunc("/ws_poll", handlers.HandleConnectionsPoll)
	go handlers.HandleMessagesPoll()
	fmt.Println("🚀 Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
