package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}


var clients = make(map[*websocket.Conn]bool)

func wsHandler(w http.ResponseWriter, r *http.Request) {
 
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("Upgrade error:", err)
        return
    }
    defer conn.Close()

    clients[conn] = true
    log.Println("New client connected")

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            log.Println("Read error:", err)
            delete(clients, conn)
            break
        }

        for client := range clients {
            if client != conn {
                err := client.WriteMessage(websocket.TextMessage, msg)
                if err != nil {
                    log.Println("Write error:", err)
                    client.Close()
                    delete(clients, client)
                }
            }
        }
    }
}


func serveHome(w http.ResponseWriter,r *http.Request) {
	      templ,err := template.ParseFiles("templates/sample.html")

		  if err != nil {
			 fmt.Println("Parsing error",err)
            }

			templ.Execute(w,nil)
}
func main() {
    http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/",serveHome)
    log.Println("Server started on :8080")
    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        log.Fatal("ListenAndServe:", err)
    }
}
