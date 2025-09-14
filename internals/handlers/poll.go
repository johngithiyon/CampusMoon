package handlers

import (
	"log"
	"net/http"
	"sync"
	"math/rand"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type           string      `json:"type"`
	Question       string      `json:"question,omitempty"`
	Options        []string    `json:"options,omitempty"`
	CorrectAnswer  int         `json:"correctAnswer,omitempty"`
	Choice         int         `json:"choice,omitempty"`
	Correct        bool        `json:"correct,omitempty"`
	PollId         string      `json:"pollId,omitempty"`
	OptionIndex    int         `json:"optionIndex,omitempty"`
	Sender         string      `json:"sender,omitempty"`
	ParticipantId  string      `json:"participantId,omitempty"`
	ParticipantName string     `json:"participantName,omitempty"`
	Poll           *PollData   `json:"poll,omitempty"`
	FinalResults   map[int]int `json:"finalResults,omitempty"`
}

type PollData struct {
	Id            string          `json:"id"`
	Question      string          `json:"question"`
	Options       []string        `json:"options"`
	CorrectAnswer int             `json:"correctAnswer"`
	Results       map[int]int     `json:"results"`
	TotalVotes    int             `json:"totalVotes"`
	Attendance    map[string]bool `json:"attendance"`
}

type Client_poll struct {
	conn *websocket.Conn
	name string
}

var (
	clientsPoll    = make(map[*Client_poll]bool)
	broadcastPoll  = make(chan Message)
	mutexPoll      = &sync.Mutex{}
	activePoll     *PollData
	attendancePoll = make(map[string]string) // participantId -> participantName
)

var upgraderPoll = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleConnectionsPoll(w http.ResponseWriter, r *http.Request) {
	ws, err := upgraderPoll.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Generate a unique ID for the participant
	participantId := "user-" + randString(8)
	client_poll := &Client_poll{conn: ws, name: "Participant " + randString(4)}
	
	mutexPoll.Lock()
	clientsPoll[client_poll] = true
	attendancePoll[participantId] = client_poll.name
	mutexPoll.Unlock()

	// Send participant their ID
	ws.WriteJSON(Message{
		Type:          "participant-id",
		ParticipantId: participantId,
	})

	// Notify others about new participant
	broadcastPoll <- Message{
		Type:           "participant-joined",
		ParticipantId:  participantId,
		ParticipantName: client_poll.name,
	}

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("❌ error: %v", err)
			mutexPoll.Lock()
			delete(clientsPoll, client_poll)
			delete(attendancePoll, participantId)
			mutexPoll.Unlock()
			break
		}

		// Set the sender ID for the message
		msg.Sender = participantId
		
		switch msg.Type {
		case "participant-updated":
			mutexPoll.Lock()
			client_poll.name = msg.ParticipantName
			attendancePoll[participantId] = msg.ParticipantName
			mutexPoll.Unlock()
			
		case "poll-created":
			mutexPoll.Lock()
			// Store the active poll
			activePoll = msg.Poll
			// Initialize attendance tracking
			activePoll.Attendance = make(map[string]bool)
			mutexPoll.Unlock()
			
		case "poll-vote":
			mutexPoll.Lock()
			if activePoll != nil && activePoll.Id == msg.PollId {
				// Update poll results
				activePoll.Results[msg.OptionIndex]++
				activePoll.TotalVotes++
				
				// Check if the answer is correct
				isCorrect := msg.OptionIndex == activePoll.CorrectAnswer
				
				// Mark attendance if answer is correct
				if isCorrect {
					activePoll.Attendance[msg.Sender] = true
					
					// Send attendance confirmation to the participant
					msg.Correct = true
					msg.Type = "attendance-marked"
					
					// Send to all clients_poll
					for client_poll := range clientsPoll {
						err := client_poll.conn.WriteJSON(msg)
						if err != nil {
							log.Printf("❌ error: %v", err)
							client_poll.conn.Close()
							delete(clientsPoll, client_poll)
						}
					}
					mutexPoll.Unlock()
					continue // Skip the normal broadcast
				}
			}
			mutexPoll.Unlock()
			
		case "poll-ended":
			mutexPoll.Lock()
			activePoll = nil
			mutexPoll.Unlock()
		}

		broadcastPoll <- msg
	}
}

func HandleMessagesPoll() {
	for {
		msg := <-broadcastPoll
		mutexPoll.Lock()
		for client_poll := range clientsPoll {
			err := client_poll.conn.WriteJSON(msg)
			if err != nil {
				log.Printf("❌ error: %v", err)
				client_poll.conn.Close()
				delete(clientsPoll, client_poll)
			}
		}
		mutexPoll.Unlock()
	}
}

func ServePoll(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/poll.html")
}

// Helper function to generate random string
func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}