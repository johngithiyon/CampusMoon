package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"
    _ "github.com/lib/pq"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ===== MinIO Setup =====
var minioClient *minio.Client
var bucketName = "videos"

// ===== Postgres Setup =====
var db *sql.DB

func initDB() {
	var err error
	connStr := "host=localhost port=5432 user=john password=john dbname=campusmoon sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalln("Failed to connect to Postgres:", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalln("Postgres ping failed:", err)
	}

	// Create table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS videos (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		filename VARCHAR(255) NOT NULL,
		uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		log.Fatalln("Failed to create table:", err)
	}

	log.Println("Connected to Postgres and table ready")
}

// ===== WebRTC Setup =====
type Client struct {
	ID   string
	Conn *websocket.Conn
}

var clients = make(map[string]*Client)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	// Initialize DB
	initDB()

	// Initialize MinIO client
	endpoint := "localhost:9000"
	accessKeyID := "john"
	secretAccessKey := "johngithiyon"
	useSSL := false

	var err error
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln("Failed to connect to MinIO:", err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Println("Bucket already exists:", bucketName)
		} else {
			log.Fatalln("Error creating bucket:", err)
		}
	}

	// ===== Routes =====
	http.HandleFunc("/", serveHome)           // index.html
	http.HandleFunc("/meet", serveMeet)       // meet.html
	http.HandleFunc("/upload", uploadHandler) // video upload
	http.HandleFunc("/videos", videosHandler) // list videos
	http.HandleFunc("/ws", handleWS)          // WebSocket for video chat

	fmt.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// ===== Frontend Handlers =====
func serveHome(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		return
	}
	templ.Execute(w, nil)
}

func serveMeet(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("meet.html")
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		return
	}
	templ.Execute(w, nil)
}

// ===== MinIO + Postgres Handlers =====
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "Failed to read file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	title := r.FormValue("title")
	description := r.FormValue("description")
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	fileName := uuid.New().String() + "_" + header.Filename

	_, err = minioClient.PutObject(context.Background(), bucketName, fileName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		http.Error(w, "Failed to upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`INSERT INTO videos (title, description, filename) VALUES ($1, $2, $3)`, title, description, fileName)
	if err != nil {
		http.Error(w, "Failed to store video info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{"success": true, "name": fileName}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func videosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	rows, err := db.Query(`SELECT title, description, filename FROM videos ORDER BY uploaded_at DESC`)
	if err != nil {
		http.Error(w, "Failed to query videos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Video struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}

	var videos []Video

	for rows.Next() {
		var title, description, filename string
		err := rows.Scan(&title, &description, &filename)
		if err != nil {
			log.Println("Row scan error:", err)
			continue
		}

		presignedURL, err := minioClient.PresignedGetObject(context.Background(), bucketName, filename, time.Hour*24, url.Values{})
		if err != nil {
			log.Println("Error generating URL:", err)
			continue
		}

		videos = append(videos, Video{
			Title:       title,
			Description: description,
			URL:         presignedURL.String(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
}

// ===== WebSocket Handlers =====
func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	id := uuid.New().String()
	client := &Client{ID: id, Conn: conn}
	clients[id] = client

	for _, c := range clients {
		if c.ID != id {
			c.Conn.WriteJSON(map[string]interface{}{"type": "new-peer", "id": id})
		}
	}

	existingPeers := []string{}
	for _, c := range clients {
		if c.ID != id {
			existingPeers = append(existingPeers, c.ID)
		}
	}
	conn.WriteJSON(map[string]interface{}{"type": "existing-peers", "peers": existingPeers})

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		toID, ok := msg["to"].(string)
		if ok {
			if c, found := clients[toID]; found {
				msg["from"] = id
				c.Conn.WriteJSON(msg)
			}
		}
	}

	delete(clients, id)
	conn.Close()
}
