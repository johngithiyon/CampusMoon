package models



import "github.com/gorilla/websocket"

type Client struct {
	ID       string
	Conn     *websocket.Conn
	UserID   string
	IsStaff  bool
	UserName string
}

type ChatMessage struct {
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	UserID    string `json:"user_id"`
}

type Video struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
}
