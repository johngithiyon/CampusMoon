package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ===== MinIO Setup =====
var minioClient *minio.Client
var bucketName = "videos"

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

// ===== MinIO Handlers =====
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Allow CORS
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

	fileName := uuid.New().String() + "_" + header.Filename

	_, err = minioClient.PutObject(context.Background(), bucketName, fileName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		http.Error(w, "Failed to upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{"success": true, "name": fileName}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func videosHandler(w http.ResponseWriter, r *http.Request) {
	// Allow CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := context.Background()
	objectCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})

	type Video struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	var videos []Video

	for object := range objectCh {
		if object.Err != nil {
			log.Println("Error listing object:", object.Err)
			continue
		}

		reqParams := make(url.Values)
		presignedURL, err := minioClient.PresignedGetObject(ctx, bucketName, object.Key, time.Hour*24, reqParams)
		if err != nil {
			log.Println("Error generating URL:", err)
			continue
		}

		videos = append(videos, Video{
			Name: object.Key,
			URL:  presignedURL.String(),
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

	// Notify existing clients about new peer
	for _, c := range clients {
		if c.ID != id {
			c.Conn.WriteJSON(map[string]interface{}{
				"type": "new-peer",
				"id":   id,
			})
		}
	}

	// Notify new client about existing peers
	existingPeers := []string{}
	for _, c := range clients {
		if c.ID != id {
			existingPeers = append(existingPeers, c.ID)
		}
	}
	conn.WriteJSON(map[string]interface{}{
		"type":  "existing-peers",
		"peers": existingPeers,
	})

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
