package handlers

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

var clients_poll = make(map[*websocket.Conn]bool)
var broadcast_poll = make(chan Message)

var upgrader_poll = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleConnectionsPoll(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader_poll.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	clients_poll[ws] = true

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("❌ error: %v", err)
			delete(clients_poll, ws)
			break
		}
		broadcast_poll <- msg
	}
}

func HandleMessagesPoll() {
	for {
		msg := <-broadcast_poll
		for client := range clients_poll {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("❌ error: %v", err)
				client.Close()
				delete(clients_poll, client)
			}
		}
	}
}

func ServePoll(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/poll.html")
}
