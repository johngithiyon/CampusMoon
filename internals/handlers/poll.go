package handlers

import (
	
	"net/http"
)

func ServePoll(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/poll.html")
}
