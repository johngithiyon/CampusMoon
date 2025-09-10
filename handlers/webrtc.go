package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
	"encoding/json" 

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

// ===== Client Struct =====
type Client struct {
	ID       string
	Conn     *websocket.Conn
	UserID   string
	IsStaff  bool
	UserName string
}

var Clients = make(map[string]*Client)
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ===== WebSocket Handler =====
func Handlewebrtc(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// Generate a unique ID
	id := uuid.New().String()
	client := &Client{ID: id, Conn: conn}
	Clients[id] = client

	defer func() {
		delete(Clients, id)
		conn.Close()
	}()

	// Send existing peers
	existingPeers := []string{}
	for peerID := range Clients {
		if peerID != id {
			existingPeers = append(existingPeers, peerID)
		}
	}
	conn.WriteJSON(map[string]interface{}{"type": "existing-peers", "peers": existingPeers})

	// Notify others about new peer
	for peerID, c := range Clients {
		if peerID != id {
			c.Conn.WriteJSON(map[string]interface{}{"type": "new-peer", "id": id})
		}
	}

	// Listen for messages
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case "user-info":
			if userID, ok := msg["userId"].(string); ok {
				client.UserID = userID
			}
			if isStaff, ok := msg["isStaff"].(bool); ok {
				client.IsStaff = isStaff
			}
			client.UserName = fmt.Sprintf("User %s", client.UserID[:6])

		case "offer", "answer", "ice-candidate":
			toID, ok := msg["to"].(string)
			if ok {
				if c, found := Clients[toID]; found {
					msg["from"] = id
					c.Conn.WriteJSON(msg)
				}
			}

		case "chat-message":
			HandleChatMessage(db, client, msg)

		default:
			log.Printf("Unknown message type: %s", msgType)
		}
	}

	// Notify others about disconnect
	for _, c := range Clients {
		c.Conn.WriteJSON(map[string]interface{}{"type": "peer-disconnected", "id": id})
	}
}

// ===== Chat Message Handling =====
func HandleChatMessage(db *sql.DB, sender *Client, msg map[string]interface{}) {
	message, ok := msg["message"].(string)
	if !ok || message == "" {
		return
	}

	timestamp, ok := msg["timestamp"].(string)
	if !ok {
		timestamp = time.Now().Format(time.RFC3339)
	}

	senderID := sender.UserID
	if senderID == "" {
		senderID = sender.ID
	}

	senderName := sender.UserName
	if senderName == "" {
		senderName = fmt.Sprintf("User %s", senderID[:6])
	}

	_, err := db.Exec(`INSERT INTO chat_messages (sender_id, sender_name, message, timestamp) 
		VALUES ($1, $2, $3, $4)`, senderID, senderName, message, timestamp)
	if err != nil {
		log.Println("Error saving chat message:", err)
	}

	chatMsg := map[string]interface{}{
		"type":      "chat-message",
		"sender":    senderName,
		"message":   message,
		"timestamp": timestamp,
		"user_id":   senderID,
	}

	for _, client := range Clients {
		if err := client.Conn.WriteJSON(chatMsg); err != nil {
			log.Println("Error sending chat message:", err)
		}
	}
}

// ===== Chat History =====
func ChatHistory(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query(`SELECT sender_id, sender_name, message, timestamp 
		FROM chat_messages 
		WHERE room_id = 'default' 
		ORDER BY timestamp DESC 
		LIMIT 50`)
	if err != nil {
		http.Error(w, "Failed to query chat messages: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Message struct {
		SenderID   string `json:"user_id"`
		SenderName string `json:"sender"`
		Message    string `json:"message"`
		Timestamp  string `json:"timestamp"`
	}

	var messages []Message
	for rows.Next() {
		var senderID, senderName, message, timestamp string
		err := rows.Scan(&senderID, &senderName, &message, &timestamp)
		if err != nil {
			log.Println("Row scan error:", err)
			continue
		}
		messages = append(messages, Message{
			SenderID:   senderID,
			SenderName: senderName,
			Message:    message,
			Timestamp:  timestamp,
		})
	}

	// Reverse order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}
