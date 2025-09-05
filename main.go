package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "html/template"
    "github.com/gorilla/websocket"
    "github.com/google/uuid"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
    ID   string
    Conn *websocket.Conn
}

var clients = make(map[string]*Client)
var broadcast = make(chan BroadcastMessage)

type Message struct {
    Type string          `json:"type"`
    Data json.RawMessage `json:"data"`
    From string          `json:"from,omitempty"`
}

type BroadcastMessage struct {
    SenderID string
    Msg      Message
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println(err)
        return
    }
    defer ws.Close()

    clientID := uuid.New().String()
    client := &Client{ID: clientID, Conn: ws}
    clients[clientID] = client

    log.Printf("Client connected: %s", clientID)

    for {
        var msg Message
        err := ws.ReadJSON(&msg)
        if err != nil {
            log.Println("error reading json:", err)
            delete(clients, clientID)
            break
        }

        msg.From = clientID
        broadcast <- BroadcastMessage{SenderID: clientID, Msg: msg}
    }
}

func handleMessages() {
    for {
        bmsg := <-broadcast
        for id, client := range clients {
            if id != bmsg.SenderID { // 🚀 don’t send back to sender
                err := client.Conn.WriteJSON(bmsg.Msg)
                if err != nil {
                    log.Printf("error: %v", err)
                    client.Conn.Close()
                    delete(clients, id)
                }
            }
        }
    }
}

func serveHome(w http.ResponseWriter, r *http.Request) {
    tmpl, err := template.ParseFiles("templates/sample.html")
    if err != nil {
        http.Error(w, "Error loading template", http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, nil)
}


func main() {
	http.HandleFunc("/",serveHome)
    http.HandleFunc("/ws", handleConnections)
    go handleMessages()
    fmt.Println("Signalling server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
