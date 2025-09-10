package handlers

import (
	"CampusMoon/internals/storage"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

func AdminAPI(w http.ResponseWriter, r *http.Request) {
	studentID := randomID("STU")
	staffID := randomID("STF")

	_, err := storage.DB.Exec("INSERT INTO user_ids (student_id, staff_id) VALUES ($1, $2)", studentID, staffID)
	if err != nil {
		http.Error(w, "Database insert failed", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"student_id": studentID, "staff_id": staffID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func randomID(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s-%04d", prefix, rand.Intn(10000))
}
