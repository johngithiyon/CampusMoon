package handlers

import (
	"CampusMoon/internals/storage"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type AudioDetail struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	SubjectID   *int64 `json:"subject_id,omitempty"`
}

type DeleteAudioRequest struct {
	Filename string `json:"filename"`
}

var AudioBucket = "audios"
var AudioPublicURLPrefix = "http://localhost:9000"

// ---------------- Upload ----------------
func UploadAudioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	subjectIDStr := r.FormValue("subject_id")

	if title == "" {
		http.Error(w, "Title required", http.StatusBadRequest)
		return
	}

	var subjectID sql.NullInt64
	if subjectIDStr != "" {
		id, err := strconv.ParseInt(subjectIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid subject_id", http.StatusBadRequest)
			return
		}
		subjectID = sql.NullInt64{Int64: id, Valid: true}
	} else {
		subjectID = sql.NullInt64{Valid: false} // NULL in DB
	}

	// Get audio file
	audioFile, audioHeader, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "Error retrieving audio file", http.StatusBadRequest)
		return
	}
	defer audioFile.Close()

	audioObjectName := uuid.New().String() + "-" + audioHeader.Filename
	audioContentType := audioHeader.Header.Get("Content-Type")

	// Upload to MinIO
	_, err = storage.MinioClient.PutObject(
		context.Background(),
		AudioBucket,
		audioObjectName,
		audioFile,
		audioHeader.Size,
		minio.PutObjectOptions{ContentType: audioContentType},
	)
	if err != nil {
		http.Error(w, "Upload audio to storage failed", http.StatusInternalServerError)
		return
	}

	audioURL := fmt.Sprintf("%s/%s/%s", AudioPublicURLPrefix, AudioBucket, audioObjectName)

	// Save metadata in DB
	var newID int64
	err = storage.DB.QueryRow(
		`INSERT INTO audios (title, description, filename, url, subject_id)
         VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		title, description, audioObjectName, audioURL, subjectID,
	).Scan(&newID)
	if err != nil {
		log.Println("DB insert error:", err)
		http.Error(w, "Database insert failed", http.StatusInternalServerError)
		return
	}

	detail := AudioDetail{
		ID:          newID,
		Title:       title,
		Description: description,
		Filename:    audioObjectName,
		URL:         audioURL,
	}
	if subjectID.Valid {
		detail.SubjectID = &subjectID.Int64
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

// ---------------- List ----------------
func AudiosHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := storage.DB.Query("SELECT id, title, description, filename, url, subject_id FROM audios ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var audios []AudioDetail
	for rows.Next() {
		var a AudioDetail
		var subjectID sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.Filename, &a.URL, &subjectID); err != nil {
			log.Println("Row scan error:", err)
			continue
		}
		if subjectID.Valid {
			a.SubjectID = &subjectID.Int64
		}
		audios = append(audios, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(audios)
}

// ---------------- Detail ----------------
func AudioDetailHandler(w http.ResponseWriter, r *http.Request) {
	idParam := r.URL.Query().Get("id")
	if idParam == "" {
		http.Error(w, "Missing id param", http.StatusBadRequest)
		return
	}

	var a AudioDetail
	var subjectID sql.NullInt64
	err := storage.DB.QueryRow(
		"SELECT id, title, description, filename, url, subject_id FROM audios WHERE id = $1",
		idParam,
	).Scan(&a.ID, &a.Title, &a.Description, &a.Filename, &a.URL, &subjectID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Audio not found", http.StatusNotFound)
		} else {
			log.Println("DB query error:", err)
			http.Error(w, "Database query failed", http.StatusInternalServerError)
		}
		return
	}
	if subjectID.Valid {
		a.SubjectID = &subjectID.Int64
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

// ---------------- Delete ----------------
func DeleteAudioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeleteAudioRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Filename == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// remove from MinIO
	err = storage.MinioClient.RemoveObject(context.Background(), AudioBucket, req.Filename, minio.RemoveObjectOptions{})
	if err != nil {
		http.Error(w, "Failed to delete audio from storage", http.StatusInternalServerError)
		return
	}

	// remove from DB
	_, err = storage.DB.Exec("DELETE FROM audios WHERE filename = $1", req.Filename)
	if err != nil {
		http.Error(w, "Failed to delete from DB", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "✅ Audio deleted successfully")
}
