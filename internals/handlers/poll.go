package handlers

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type         string                 `json:"type"`
	UserID       string                 `json:"userId,omitempty"`
	IsStaff      bool                   `json:"isStaff,omitempty"`
	From         string                 `json:"from,omitempty"`
	To           string                 `json:"to,omitempty"`
	Offer        map[string]interface{} `json:"offer,omitempty"`
	Answer       map[string]interface{} `json:"answer,omitempty"`
	Candidate    map[string]interface{} `json:"candidate,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Sender       string                 `json:"sender,omitempty"`
	Timestamp    string                 `json:"timestamp,omitempty"`
	Poll         *Poll                  `json:"poll,omitempty"`
	Results      map[string]int         `json:"results,omitempty"`
	FinalResults map[string]int         `json:"finalResults,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Peers        []string               `json:"peers,omitempty"`
	PollID       string                 `json:"pollId,omitempty"`
}

type Poll struct {
	ID       string         `json:"id"`
	Question string         `json:"question"`
	Options  []string       `json:"options"`
	Results  map[string]int `json:"results"`
}

var upgrader_poll = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client_poll struct {
	ID      string
	IsStaff bool
	Conn    *websocket.Conn
}

var (
	clients_poll  = make(map[string]*Client_poll)
	clientsMutex  sync.Mutex
	polls         = make(map[string]*Poll)
)

func Handlepoll(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader_poll.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	var client_poll *Client_poll

	defer func() {
		if client_poll != nil {
			removeClient(client_poll.ID)
		}
		conn.Close()
	}()

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		switch msg.Type {
		case "user-info":
			client_poll = &Client_poll{
				ID:      msg.UserID,
				IsStaff: msg.IsStaff,
				Conn:    conn,
			}
			addClient(client_poll)
			sendExistingPeers(client_poll)
			broadcast_poll(Message{Type: "new-peer", ID: client_poll.ID}, client_poll.ID)

		case "offer", "answer", "ice-candidate":
			if targetClient, ok := getClient(msg.To); ok {
				targetClient.Conn.WriteJSON(msg)
			}

		case "chat-message":
			broadcast_poll(Message{
				Type:      "chat-message",
				Sender:    msg.Sender,
				Message:   msg.Message,
				Timestamp: msg.Timestamp,
			}, "")

		case "poll-created":
			if msg.Poll != nil {
				polls[msg.Poll.ID] = msg.Poll
				broadcast_poll(Message{
					Type: "poll-created",
					Poll: msg.Poll,
				}, "")
			}

		case "poll-vote":
			if poll, ok := polls[msg.Poll.ID]; ok {
				for _, option := range poll.Options {
					if option == msg.Message { // msg.Message contains the chosen option
						if poll.Results == nil {
							poll.Results = make(map[string]int)
						}
						poll.Results[option]++
						break
					}
				}
				broadcast_poll(Message{
					Type:    "poll-vote",
					PollID:  poll.ID,
					Results: poll.Results,
				}, "")
			}

		case "poll-ended":
			if poll, ok := polls[msg.Poll.ID]; ok {
				broadcast_poll(Message{
					Type:        "poll-ended",
					PollID:      poll.ID,
					FinalResults: poll.Results,
				}, "")
				delete(polls, poll.ID)
			}
		}
	}
}

func addClient(client_poll *Client_poll) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	clients_poll[client_poll.ID] = client_poll
}

func removeClient(clientID string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	delete(clients_poll, clientID)
	broadcast_poll(Message{Type: "peer-disconnected", ID: clientID}, "")
}

func getClient(clientID string) (*Client_poll, bool) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	c, ok := clients_poll[clientID]
	return c, ok
}

func broadcast_poll(msg Message, excludeID string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for id, c := range clients_poll {
		if id == excludeID {
			continue
		}
		c.Conn.WriteJSON(msg)
	}
}

func sendExistingPeers(client_poll *Client_poll) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	peerIDs := []string{}
	for id := range clients_poll {
		if id != client_poll.ID {
			peerIDs = append(peerIDs, id)
		}
	}
	client_poll.Conn.WriteJSON(Message{
		Type:  "existing-peers",
		Peers: peerIDs,
	})
}
