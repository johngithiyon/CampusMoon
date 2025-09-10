// main.go
package main

import (
	
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type     string   `json:"type"`
	Question string   `json:"question,omitempty"`
	Options  []string `json:"options,omitempty"`
	Answer   int      `json:"answer,omitempty"`
	Choice   int      `json:"choice,omitempty"`
	Correct  bool     `json:"correct,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

func main() {
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	clients[ws] = true

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Println("Error:", err)
			delete(clients, ws)
			break
		}
		broadcast <- msg
	}
}

func handleMessages() {
	var currentPoll Message

	for {
		msg := <-broadcast

		if msg.Type == "createPoll" {
			currentPoll = msg
			// Broadcast poll to everyone
			for client := range clients {
				client.WriteJSON(currentPoll)
			}
		}

		if msg.Type == "vote" {
			// Check answer
			msg.Correct = (msg.Choice == currentPoll.Answer)
			// Send result back only to voter
			for client := range clients {
				client.WriteJSON(msg)
			}
		}
	}
}
