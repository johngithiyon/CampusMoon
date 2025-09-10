package handlers

import (
	"CampusMoon/internals/models"
	"CampusMoon/internals/storage"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read form data
	title := r.FormValue("title")
	description := r.FormValue("description")

	// File
	file, handler, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	objectName := uuid.New().String() + "-" + handler.Filename
	contentType := handler.Header.Get("Content-Type")

	// Upload to MinIO
	_, err = storage.MinioClient.PutObject(
		context.Background(),
		storage.BucketName,
		objectName,
		file,
		handler.Size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		http.Error(w, "Upload to MinIO failed", http.StatusInternalServerError)
		return
	}

	// Save metadata to DB
	_, err = storage.DB.Exec(
		"INSERT INTO videos (title, description, filename) VALUES ($1, $2, $3)",
		title, description, objectName,
	)
	if err != nil {
		http.Error(w, "Database insert failed", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "✅ Video uploaded successfully")
}

func VideosHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := storage.DB.Query("SELECT title, description, filename FROM videos ORDER BY uploaded_at DESC")
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var filename string
		if err := rows.Scan(&v.Title, &v.Description, &filename); err != nil {
			continue
		}
		v.URL = fmt.Sprintf("http://localhost:9000/%s/%s", storage.BucketName, filename)
		videos = append(videos, v)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
}
