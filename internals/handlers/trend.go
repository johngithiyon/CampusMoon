package handlers

import (
	"CampusMoon/internals/storage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
)

// ImageUploadResponse represents the response after image upload
type ImageUploadResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ImageURL  string `json:"imageUrl"`
	ImageName string `json:"imageName"`
	ImageID   int    `json:"imageId"`
}

// ImageInfo represents image metadata
type ImageInfo struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Size        int64     `json:"size"`
	UploadedAt  time.Time `json:"uploadedAt"`
	LikeCount   int       `json:"likeCount"`
	CommentCount int      `json:"commentCount"`
	UserLiked   bool      `json:"userLiked"`
}

// LikeRequest represents a like/unlike request
type LikeRequest struct {
	ImageID int    `json:"imageId"`
	UserID  string `json:"userId"`
}

// CommentRequest represents a comment request
type CommentRequest struct {
	ImageID int    `json:"imageId"`
	UserID  string `json:"userId"`
	Comment string `json:"comment"`
}

// CommentResponse represents a comment
type CommentResponse struct {
	ID        int       `json:"id"`
	UserID    string    `json:"userId"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

// UploadImageHandler handles image uploads to MinIO
func UploadImageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse multipart form (10MB max)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		sendErrorResponse(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get user ID from form or generate one
	userID := r.FormValue("userId")
	if userID == "" {
		userID = generateUserID()
	}

	// Get the file from form data
	file, handler, err := r.FormFile("image")
	if err != nil {
		sendErrorResponse(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	ext := strings.ToLower(filepath.Ext(handler.Filename))
	if !allowedExtensions[ext] {
		sendErrorResponse(w, "Invalid file type. Allowed: JPG, JPEG, PNG, GIF, WEBP", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	timestamp := time.Now().UnixNano()
	uniqueName := fmt.Sprintf("%d%s", timestamp, ext)

	// Upload to MinIO
	ctx := context.Background()
	_, err = storage.MinioClient.PutObject(
		ctx,
		storage.ImageBucketName,
		uniqueName,
		file,
		handler.Size,
		minio.PutObjectOptions{
			ContentType: handler.Header.Get("Content-Type"),
		},
	)

	if err != nil {
		log.Printf("Error uploading image to MinIO: %v", err)
		sendErrorResponse(w, "Failed to upload image", http.StatusInternalServerError)
		return
	}

	// Generate URL for the uploaded image
	imageURL := fmt.Sprintf("/image/%s", uniqueName)

	// Save image metadata to database
	var imageID int
	err = storage.DB.QueryRow(
		"INSERT INTO images (user_id, filename, url) VALUES ($1, $2, $3) RETURNING id",
		userID, uniqueName, imageURL,
	).Scan(&imageID)

	if err != nil {
		log.Printf("Error saving image metadata: %v", err)
		sendErrorResponse(w, "Failed to save image metadata", http.StatusInternalServerError)
		return
	}

	response := ImageUploadResponse{
		Success:   true,
		Message:   "Image uploaded successfully",
		ImageURL:  imageURL,
		ImageName: uniqueName,
		ImageID:   imageID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetImageHandler serves images from MinIO
func GetImageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	imageName := vars["imageName"]

	if imageName == "" {
		http.Error(w, "Image name required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	object, err := storage.MinioClient.GetObject(
		ctx,
		storage.ImageBucketName,
		imageName,
		minio.GetObjectOptions{},
	)

	if err != nil {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}
	defer object.Close()

	// Get object info to set proper content type
	objInfo, err := object.Stat()
	if err != nil {
		http.Error(w, "Error getting image info", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", objInfo.ContentType)
	w.Header().Set("Cache-Control", "max-age=3600") // Cache for 1 hour

	// Stream the image to response
	io.Copy(w, object)
}

// ListImagesHandler returns list of all uploaded images with like/comment counts
func ListImagesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = generateUserID()
	}

	query := `
		SELECT 
			i.id, i.filename, i.url, i.uploaded_at,
			COALESCE(l.like_count, 0) as like_count,
			COALESCE(c.comment_count, 0) as comment_count,
			EXISTS(SELECT 1 FROM image_likes WHERE image_id = i.id AND user_id = $1) as user_liked
		FROM images i
		LEFT JOIN (
			SELECT image_id, COUNT(*) as like_count 
			FROM image_likes 
			GROUP BY image_id
		) l ON i.id = l.image_id
		LEFT JOIN (
			SELECT image_id, COUNT(*) as comment_count 
			FROM image_comments 
			GROUP BY image_id
		) c ON i.id = c.image_id
		ORDER BY i.uploaded_at DESC
	`

	rows, err := storage.DB.Query(query, userID)
	if err != nil {
		log.Printf("Error querying images: %v", err)
		sendErrorResponse(w, "Error loading images", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var images []ImageInfo
	for rows.Next() {
		var image ImageInfo
		var uploadedAt time.Time
		err := rows.Scan(&image.ID, &image.Name, &image.URL, &uploadedAt, 
			&image.LikeCount, &image.CommentCount, &image.UserLiked)
		if err != nil {
			log.Printf("Error scanning image row: %v", err)
			continue
		}
		image.UploadedAt = uploadedAt
		images = append(images, image)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

// DeleteImageHandler deletes an image from MinIO and database
func DeleteImageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	imageIDStr := r.URL.Query().Get("id")
	imageID, err := strconv.Atoi(imageIDStr)
	if err != nil {
		sendErrorResponse(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	// Get image filename from database
	var filename string
	err = storage.DB.QueryRow("SELECT filename FROM images WHERE id = $1", imageID).Scan(&filename)
	if err != nil {
		sendErrorResponse(w, "Image not found", http.StatusNotFound)
		return
	}

	// Delete from MinIO
	ctx := context.Background()
	err = storage.MinioClient.RemoveObject(ctx, storage.ImageBucketName, filename, minio.RemoveObjectOptions{})
	if err != nil {
		log.Printf("Error deleting image from MinIO: %v", err)
	}

	// Delete from database (cascade will handle likes and comments)
	_, err = storage.DB.Exec("DELETE FROM images WHERE id = $1", imageID)
	if err != nil {
		sendErrorResponse(w, "Failed to delete image", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Image deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LikeImageHandler handles liking an image
func LikeImageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req LikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = generateUserID()
	}

	// Check if already liked
	var exists bool
	err := storage.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM image_likes WHERE image_id = $1 AND user_id = $2)",
		req.ImageID, req.UserID,
	).Scan(&exists)

	if err != nil {
		sendErrorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}

	if exists {
		sendErrorResponse(w, "Already liked", http.StatusBadRequest)
		return
	}

	// Add like
	_, err = storage.DB.Exec(
		"INSERT INTO image_likes (image_id, user_id) VALUES ($1, $2)",
		req.ImageID, req.UserID,
	)

	if err != nil {
		sendErrorResponse(w, "Failed to like image", http.StatusInternalServerError)
		return
	}

	// Get updated like count
	var likeCount int
	err = storage.DB.QueryRow(
		"SELECT COUNT(*) FROM image_likes WHERE image_id = $1",
		req.ImageID,
	).Scan(&likeCount)

	if err != nil {
		likeCount = 0
	}

	response := map[string]interface{}{
		"success":   true,
		"message":   "Image liked successfully",
		"likeCount": likeCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UnlikeImageHandler handles unliking an image
func UnlikeImageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req LikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = generateUserID()
	}

	// Remove like
	_, err := storage.DB.Exec(
		"DELETE FROM image_likes WHERE image_id = $1 AND user_id = $2",
		req.ImageID, req.UserID,
	)

	if err != nil {
		sendErrorResponse(w, "Failed to unlike image", http.StatusInternalServerError)
		return
	}

	// Get updated like count
	var likeCount int
	err = storage.DB.QueryRow(
		"SELECT COUNT(*) FROM image_likes WHERE image_id = $1",
		req.ImageID,
	).Scan(&likeCount)

	if err != nil {
		likeCount = 0
	}

	response := map[string]interface{}{
		"success":   true,
		"message":   "Image unliked successfully",
		"likeCount": likeCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AddCommentHandler handles adding comments to images
func AddCommentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = generateUserID()
	}

	if req.Comment == "" {
		sendErrorResponse(w, "Comment cannot be empty", http.StatusBadRequest)
		return
	}

	// Add comment
	var commentID int
	err := storage.DB.QueryRow(
		"INSERT INTO image_comments (image_id, user_id, comment) VALUES ($1, $2, $3) RETURNING id",
		req.ImageID, req.UserID, req.Comment,
	).Scan(&commentID)

	if err != nil {
		sendErrorResponse(w, "Failed to add comment", http.StatusInternalServerError)
		return
	}

	// Get comment count
	var commentCount int
	err = storage.DB.QueryRow(
		"SELECT COUNT(*) FROM image_comments WHERE image_id = $1",
		req.ImageID,
	).Scan(&commentCount)

	if err != nil {
		commentCount = 0
	}

	response := map[string]interface{}{
		"success":     true,
		"message":     "Comment added successfully",
		"commentId":   commentID,
		"commentCount": commentCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCommentsHandler returns comments for an image
func GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	imageIDStr := r.URL.Query().Get("imageId")
	if imageIDStr == "" {
		sendErrorResponse(w, "Image ID is required", http.StatusBadRequest)
		return
	}

	imageID, err := strconv.Atoi(imageIDStr)
	if err != nil {
		sendErrorResponse(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	// Verify image exists
	var imageExists bool
	err = storage.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM images WHERE id = $1)", imageID).Scan(&imageExists)
	if err != nil || !imageExists {
		sendErrorResponse(w, "Image not found", http.StatusNotFound)
		return
	}

	rows, err := storage.DB.Query(`
		SELECT id, user_id, comment, created_at 
		FROM image_comments 
		WHERE image_id = $1 
		ORDER BY created_at DESC
	`, imageID)

	if err != nil {
		log.Printf("Error querying comments: %v", err)
		sendErrorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var comments []CommentResponse
	for rows.Next() {
		var comment CommentResponse
		var createdAt time.Time
		err := rows.Scan(&comment.ID, &comment.UserID, &comment.Comment, &createdAt)
		if err != nil {
			log.Printf("Error scanning comment: %v", err)
			continue
		}
		comment.CreatedAt = createdAt
		comments = append(comments, comment)
	}

	// Always return an array, even if empty
	if comments == nil {
		comments = []CommentResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// ServeTrend serves the trend.html page
func ServeTrend(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/trend.html")
}

// Helper function to generate user ID
func generateUserID() string {
	return fmt.Sprintf("user_%d", time.Now().UnixNano())
}

