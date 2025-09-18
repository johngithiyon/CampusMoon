package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client
var bucketName = "audios"

// InitAudio sets up MinIO and ensures the bucket exists
func InitAudio() {
	endpoint := "localhost:9000"
	accessKey := "john"          // change to env var later
	secretKey := "johngithiyon"  // change to env var later
	useSSL := false

	var err error
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Created bucket:", bucketName)
	}
}

// RegisterAudioRoutes adds audio endpoints to the router
func RegisterAudioRoutes(r *mux.Router) {
	r.HandleFunc("/upload-audio", uploadHandler).Methods("POST")
	r.HandleFunc("/list-audio", listHandler).Methods("GET")

	// Serve frontend files from ./static/audio
	fs := http.FileServer(http.Dir("./static/audio"))
	r.PathPrefix("/audio/").Handler(http.StripPrefix("/audio/", fs))
}

// Upload audio
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ctx := context.Background()
	_, err = minioClient.PutObject(ctx, bucketName, handler.Filename, file, -1,
		minio.PutObjectOptions{ContentType: "audio/mpeg"})
	if err != nil {
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Uploaded successfully"))
}

// List audio files
func listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	objectCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})

	var result string
	for obj := range objectCh {
		if obj.Err != nil {
			log.Println(obj.Err)
			continue
		}
		url, err := minioClient.PresignedGetObject(ctx, bucketName, obj.Key, time.Hour, nil)
		if err == nil {
			result += fmt.Sprintf("%s\n", url.String())
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(result))
}
