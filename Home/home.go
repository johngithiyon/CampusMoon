package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client
var bucketName = "videos"

func main() {
	endpoint := "localhost:9000"
	accessKeyID := "john"       // Use your MinIO credentials
	secretAccessKey := "johngithiyon"   
	useSSL := false

	var err error
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln("Failed to initialize MinIO:", err)
	}

	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatalln("Failed to check bucket:", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatalln("Failed to create bucket:", err)
		}
		fmt.Println("Bucket created:", bucketName)
	}

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/videos", listVideosHandler)

	fmt.Println("Server started at :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
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

	file, header, err := r.FormFile("video")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := uuid.New().String() + "_" + header.Filename

	ctx := context.Background()
	info, err := minioClient.PutObject(ctx, bucketName, fileName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to upload: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Successfully uploaded %s of size %d\n", info.Key, info.Size)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"success": true, "filename":"%s"}`, fileName)))
}

// List all videos in the bucket
func listVideosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := context.Background()
	objectCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	var videos []string
	for obj := range objectCh {
		if obj.Err != nil {
			log.Println("Error listing object:", obj.Err)
			continue
		}
		videos = append(videos, obj.Key)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("index.html")
	if err != nil {
		fmt.Println("Parsing error:", err)
		return
	}
	if err := templ.Execute(w, nil); err != nil {
		log.Println("Template execution error:", err)
	}
}
