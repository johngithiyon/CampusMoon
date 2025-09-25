package storage

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client
var VideoBucketName = "videos"
var ImageBucketName = "photogram" // New bucket for images

func InitMinIO() {
	load := godotenv.Load()
	if load != nil {
		log.Println("❌ Error loading .env file")
	}

	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKeyID := os.Getenv("MINIO_ACCESS_KEY")
	secretAccessKey := os.Getenv("MINIO_SECRET_KEY")
	useSSL := false

	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln("Failed to connect to MinIO:", err)
	}

	ctx := context.Background()
	
	// Ensure video bucket exists
	ensureBucketExists(ctx, VideoBucketName)
	
	// Ensure image bucket exists
	ensureBucketExists(ctx, ImageBucketName)
	
	log.Println("✅ MinIO ready, buckets:", VideoBucketName, "and", ImageBucketName)
}

func ensureBucketExists(ctx context.Context, bucketName string) {
	err := MinioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		exists, errBucketExists := MinioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Println("Bucket already exists:", bucketName)
		} else {
			log.Fatalln("Error creating bucket:", err)
		}
	} else {
		log.Println("✅ Bucket created:", bucketName)
	}
}