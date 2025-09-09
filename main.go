package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
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

	// Videos table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS videos (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		filename VARCHAR(255) NOT NULL,
		uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		log.Fatalln("Failed to create videos table:", err)
	}

	// Admin table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS admins (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100),
		code VARCHAR(50),
		address TEXT
	);`)
	if err != nil {
		log.Fatalln("Failed to create admins table:", err)
	}

	// User IDs table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS user_ids (
		id SERIAL PRIMARY KEY,
		student_id VARCHAR(20) UNIQUE NOT NULL,
		staff_id VARCHAR(20) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		log.Fatalln("Failed to create user_ids table:", err)
	}

	log.Println("✅ Connected to Postgres and tables ready")
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

	// Admin & Auth routes
	http.HandleFunc("/welcome", servewelcome)
	http.HandleFunc("/admin", serveadmin)          // serve admin.html
	http.HandleFunc("/admin/register", adminAPI)   // handle admin form submission
	http.HandleFunc("/student", servestudent)
	http.HandleFunc("/staff", servestaff)
	http.HandleFunc("/login", loginHandler) // ✅ Staff & Student login


	fmt.Println("🚀 Server running at http://localhost:8080")
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

// ===== Templates =====
func servewelcome(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("welcome.html")
	if err != nil {
		fmt.Println("Template error in welcome page", err)
	}
	templ.Execute(w, nil)
}

func serveadmin(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("admin.html")
	if err != nil {
		fmt.Println("Template error in admin page", err)
	}
	templ.Execute(w, nil)
}

func servestaff(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("staff.html")
	if err != nil {
		fmt.Println("Template error in staff page", err)
	}
	templ.Execute(w, nil)
}

func servestudent(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("student.html")
	if err != nil {
		fmt.Println("Template error in student page", err)
	}
	templ.Execute(w, nil)
}

// ===== Admin ID Generation API =====
func randomID(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s-%06d", prefix, rand.Intn(1000000))
}

func adminAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Name    string `json:"name"`
		Code    string `json:"code"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Save admin info
	_, err := db.Exec(`INSERT INTO admins (name, code, address) VALUES ($1, $2, $3)`, data.Name, data.Code, data.Address)
	if err != nil {
		http.Error(w, "DB insert error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate Student & Staff IDs
	studentID := randomID("STU")
	staffID := randomID("STF")

	_, err = db.Exec(`INSERT INTO user_ids (student_id, staff_id) VALUES ($1, $2)`, studentID, staffID)
	if err != nil {
		http.Error(w, "DB insert error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{
		"student_id": studentID,
		"staff_id":   staffID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ===== Login API =====
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var studentID, staffID string
	err := db.QueryRow(`SELECT student_id, staff_id FROM user_ids 
		WHERE student_id = $1 OR staff_id = $1`, data.UserID).Scan(&studentID, &staffID)

	if err == sql.ErrNoRows {
		http.Error(w, "Invalid ID", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Decide role
	if data.UserID == studentID {
		resp := map[string]string{"role": "student", "redirect": "/student"}
		json.NewEncoder(w).Encode(resp)
		return
	}
	if data.UserID == staffID {
		resp := map[string]string{"role": "staff", "redirect": "/staff"}
		json.NewEncoder(w).Encode(resp)
		return
	}
}