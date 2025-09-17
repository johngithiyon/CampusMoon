package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	// Allow all origins for dev. Lock this down in production.
	discussionUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// connected clients + lock
	connectedClients = make(map[*websocket.Conn]string)
	clientsLock      sync.Mutex

	// broadcast queue
	broadcastChan = make(chan ChatMessage, 64)

	// global DB reference (set by InitDiscussion)
	globalDiscussionDB *sql.DB
)

// ChatMessage supports text and audio messages
type ChatMessage struct {
	Type      string    `json:"type"`       // "text" or "audio"
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`    // text or public audio URL
	Timestamp time.Time `json:"timestamp"`
}

// InitDiscussion initializes the discussion subsystem with DB and starts broadcaster.
// Call this once from main (pass your *sql.DB).
func InitDiscussion(db *sql.DB) {
	globalDiscussionDB = db
	go broadcaster()
}

// ServeDiscussionPage serves the HTML UI (templates/discussion.html)
func ServeDiscussionPage(w http.ResponseWriter, r *http.Request) {
	// Use ParseFiles if you want template injection later. For static file:
	http.ServeFile(w, r, filepath.Join("templates", "discussion.html"))
}

// HandleDiscussionWS upgrades to websocket and processes incoming messages.
// text frames: JSON ChatMessage {type:"text", sender:"...", content:"..."} (sender will be overwritten)
// binary frames: raw audio bytes (server will auto-detect format and upload to MinIO)
func HandleDiscussionWS(w http.ResponseWriter, r *http.Request) {
	ws, err := discussionUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}
	// ensure socket closed and client removed on return
	defer func() {
		clientsLock.Lock()
		delete(connectedClients, ws)
		clientsLock.Unlock()
		ws.Close()
	}()

	// assign a temporary username
	clientsLock.Lock()
	username := fmt.Sprintf("User%d", len(connectedClients)+1)
	connectedClients[ws] = username
	clientsLock.Unlock()

	log.Printf("%s connected to discussion", username)

	for {
		msgType, payload, err := ws.ReadMessage()
		if err != nil {
			// normal disconnect
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("%s disconnected", username)
			} else {
				log.Printf("ws read error for %s: %v", username, err)
			}
			return
		}

		switch msgType {
		case websocket.TextMessage:
			var incoming ChatMessage
			if err := json.Unmarshal(payload, &incoming); err != nil {
				log.Println("invalid text message json:", err)
				continue
			}
			// enforce server-set sender & timestamp
			incoming.Sender = username
			incoming.Timestamp = time.Now()
			if incoming.Type == "" {
				incoming.Type = "text"
			}
			// persist and broadcast
			if err := saveMessageToDB(incoming); err != nil {
				log.Println("save message:", err)
			}
			broadcastChan <- incoming

		case websocket.BinaryMessage:
			// treat as audio bytes
			audioURL, contentType, err := uploadAudioBytesToMinIO(payload)
			if err != nil {
				log.Println("upload audio:", err)
				continue
			}

			chat := ChatMessage{
				Type:      "audio",
				Sender:    username,
				Content:   audioURL,
				Timestamp: time.Now(),
			}
			// persist and broadcast
			if err := saveMessageToDB(chat); err != nil {
				log.Println("save audio message:", err)
			}
			// include content type in log (optional)
			log.Printf("uploaded audio (content-type=%s) => %s", contentType, audioURL)
			broadcastChan <- chat

		default:
			// ignore other frame types
		}
	}
}

// broadcaster sends messages from channel to all connected clients.
func broadcaster() {
	for msg := range broadcastChan {
		clientsLock.Lock()
		for client := range connectedClients {
			if err := client.WriteJSON(msg); err != nil {
				// on error, close and remove client
				log.Println("ws write error, removing client:", err)
				client.Close()
				delete(connectedClients, client)
			}
		}
		clientsLock.Unlock()
	}
}

// saveMessageToDB persists a ChatMessage into chat_history table.
// If DB not configured, it's a no-op (useful for dev).
func saveMessageToDB(m ChatMessage) error {
	if globalDiscussionDB == nil {
		return nil
	}
	// Ensure table columns match these names: sender, type, content, timestamp
	_, err := globalDiscussionDB.Exec(
		`INSERT INTO chat_history (sender, type, content, timestamp) VALUES ($1, $2, $3, $4)`,
		m.Sender, m.Type, m.Content, m.Timestamp,
	)
	return err
}

// ChatHistoryHandler serves stored history as JSON. Uses global DB.
func ChatHistoryHandler_discussion(w http.ResponseWriter, r *http.Request) {
	if globalDiscussionDB == nil {
		// return empty list rather than error in dev
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ChatMessage{})
		return
	}

	rows, err := globalDiscussionDB.Query(
		`SELECT sender, type, content, timestamp FROM chat_history ORDER BY timestamp ASC`,
	)
	if err != nil {
		log.Println("chat history query:", err)
		http.Error(w, "Failed to fetch chat history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	history := make([]ChatMessage, 0)
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.Sender, &m.Type, &m.Content, &m.Timestamp); err != nil {
			log.Println("scan history row:", err)
			continue
		}
		history = append(history, m)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// uploadAudioBytesToMinIO uploads raw bytes to MinIO bucket and returns a public URL and content-type.
// It attempts to detect the audio format by magic bytes ("OggS", EBML for webm, "RIFF" for wav).
func uploadAudioBytesToMinIO(data []byte) (publicURL string, contentType string, err error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	if endpoint == "" || accessKey == "" || secretKey == "" {
		return "", "", fmt.Errorf("minio credentials not set in environment")
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // set true if using TLS
	})
	if err != nil {
		return "", "", fmt.Errorf("minio new: %w", err)
	}

	bucket := os.Getenv("MINIO_AUDIO_BUCKET")
	if bucket == "" {
		bucket = "videos"
	}

	// auto-detect format
	ext := ".bin"
	ct := "application/octet-stream"
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("OggS")) {
		ext = ".ogg"
		ct = "audio/ogg"
	} else if len(data) >= 4 && bytes.HasPrefix(data, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		// EBML (webm / mkv family)
		ext = ".webm"
		ct = "audio/webm"
	} else if len(data) >= 4 && bytes.HasPrefix(data, []byte("RIFF")) {
		ext = ".wav"
		ct = "audio/wav"
	} else {
		// try to sniff "ftyp" for some mp4/aac containers
		if len(data) > 8 && bytes.Contains(data[:12], []byte("ftyp")) {
			ext = ".m4a"
			ct = "audio/mp4"
		}
	}

	objectName := fmt.Sprintf("audio-%s%s", uuid.New().String(), ext)
	ctx := context.Background()

	// create bucket if not exists
	exists, err := minioClient.BucketExists(ctx, bucket)
	if err != nil {
		return "", "", fmt.Errorf("bucket exists check: %w", err)
	}
	if !exists {
		if err := minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			// ignore "already exists" style race conditions
			return "", "", fmt.Errorf("make bucket: %w", err)
		}
	}

	// Put object
	reader := bytes.NewReader(data)
	_, err = minioClient.PutObject(ctx, bucket, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: ct,
	})
	if err != nil {
		return "", "", fmt.Errorf("put object: %w", err)
	}

	// Construct public URL. If MinIO is behind gateway or uses a different host for public access,
	// change accordingly; here we assume MINIO_ENDPOINT is host:port.
	publicURL = fmt.Sprintf("http://%s/%s/%s", endpoint, bucket, objectName)
	return publicURL, ct, nil
}
