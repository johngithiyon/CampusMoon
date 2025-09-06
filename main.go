package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

type Client struct {
	ID   string
	Conn *websocket.Conn
}

var clients = make(map[string]*Client)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("templates/meet.html")
	if err != nil {
		fmt.Println("Parsing error:", err)
		return
	}
	if err := templ.Execute(w, nil); err != nil {
		log.Println("Template execution error:", err)
	}
}

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleWS)

	fmt.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	id := uuid.New().String()
	client := &Client{ID: id, Conn: conn}
	clients[id] = client

	// Notify existing clients about the new peer
	for _, c := range clients {
		if c.ID != id {
			c.Conn.WriteJSON(map[string]interface{}{
				"type": "new-peer",
				"id":   id,
			})
		}
	}

	// Notify the new client about existing peers
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
