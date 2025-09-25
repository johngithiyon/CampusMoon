package handlers

import (
	"CampusMoon/internals/storage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
)

var bucketName = "livevideo"

// ✅ Upload a video and return a presigned URL
func RecorduploadHandler(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if storage.MinioClient == nil {
		http.Error(w, "MinIO client not initialized", http.StatusInternalServerError)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100 MB
	if err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "File error: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "upload-*.webm")
	if err != nil {
		http.Error(w, "Temp file error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Copy error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tempFile.Sync()

	objectName := fmt.Sprintf("recording_%d_%s", time.Now().Unix(), header.Filename)

	ctx := context.Background()
	info, err := storage.MinioClient.FPutObject(ctx, bucketName, objectName, tempFile.Name(), minio.PutObjectOptions{
		ContentType: "video/webm",
	})
	if err != nil {
		http.Error(w, "Upload error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ✅ Generate presigned URL (valid for 24 hours)
	url, err := storage.MinioClient.PresignedGetObject(ctx, bucketName, objectName, time.Hour*24, nil)
	if err != nil {
		http.Error(w, "Error generating URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("✅ Uploaded %s of size %d\n", objectName, info.Size)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"url":      url.String(), // ✅ Direct video link
		"filename": objectName,
	})
}

// ✅ List all live videos with presigned URLs
func LiveVideosHandler(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := context.Background()

	// List objects in the livevideo bucket
	objectCh := storage.MinioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{})

	var videos []map[string]string
	for object := range objectCh {
		if object.Err != nil {
			http.Error(w, object.Err.Error(), http.StatusInternalServerError)
			return
		}

		// ✅ Generate presigned URL
		url, err := storage.MinioClient.PresignedGetObject(ctx, bucketName, object.Key, time.Hour*24, nil)
		if err != nil {
			http.Error(w, "Error generating URL: "+err.Error(), http.StatusInternalServerError)
			return
		}

		video := map[string]string{
			"filename": object.Key,
			"url":      url.String(), // ✅ Direct video link
			"title":    object.Key,
		}
		videos = append(videos, video)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
}
