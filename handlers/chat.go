package handlers

import (
	"CampusMoon/models"
	"CampusMoon/storage"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	clients   = make(map[*models.Client]bool)
	broadcast = make(chan models.ChatMessage)
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mutex     = sync.Mutex{}
)

func HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
		return
	}

	userID := r.URL.Query().Get("user_id")
	isStaff := r.URL.Query().Get("role") == "staff"
	username := r.URL.Query().Get("username")

	client := &models.Client{Conn: conn, UserID: userID, IsStaff: isStaff, UserName: username}

	mutex.Lock()
	clients[client] = true
	mutex.Unlock()

	// Listen for messages
	for {
		var msg models.ChatMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			mutex.Lock()
			delete(clients, client)
			mutex.Unlock()
			conn.Close()
			break
		}

		msg.Timestamp = time.Now().Format("15:04:05")

		// Save in DB
		_, err = storage.DB.Exec(
			"INSERT INTO chat_messages (sender_id, sender_name, message) VALUES ($1,$2,$3)",
			client.UserID, client.UserName, msg.Message,
		)
		if err != nil {
			log.Println("❌ Error saving chat:", err)
		}

		broadcast <- msg
	}
}

func ChatHistoryHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := storage.DB.Query("SELECT sender_name, message, timestamp FROM chat_messages ORDER BY timestamp ASC LIMIT 50")
	if err != nil {
		http.Error(w, "DB query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []models.ChatMessage
	for rows.Next() {
		var msg models.ChatMessage
		var ts sql.NullTime
		if err := rows.Scan(&msg.Sender, &msg.Message, &ts); err == nil {
			if ts.Valid {
				msg.Timestamp = ts.Time.Format("15:04:05")
			}
			history = append(history, msg)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// Goroutine to broadcast
func InitChat() {
	for {
		msg := <-broadcast
		mutex.Lock()
		for client := range clients {
			if err := client.Conn.WriteJSON(msg); err != nil {
				client.Conn.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}
