package handlers

import (

    "CampusMoon/internals/storage"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/google/uuid"
    "github.com/minio/minio-go/v7"
	"database/sql"
)

type DeleteRequest struct {
    Filename string `json:"filename"`
}

type VideoDetail struct {
    ID            int64  `json:"id"`
    Title         string `json:"title"`
    Description   string `json:"description"`
    Filename      string `json:"filename"`
    URL           string `json:"url"`
    NotesFilename string `json:"notes_filename,omitempty"`
    NotesURL      string `json:"notes_url,omitempty"`
}

var PublicURLPrefix = "http://localhost:9000"


// UploadHandler handles video + optional notes upload
func UploadHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Expect multipart form
    err := r.ParseMultipartForm(100 << 20) // e.g. up to ~100MB, adjust
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    title := r.FormValue("title")
    description := r.FormValue("description")
    if title == "" {
        http.Error(w, "Title required", http.StatusBadRequest)
        return
    }

    // video file
    videoFile, videoHeader, err := r.FormFile("video")
    if err != nil {
        http.Error(w, "Error retrieving video file", http.StatusBadRequest)
        return
    }
    defer videoFile.Close()

    videoObjectName := uuid.New().String() + "-" + videoHeader.Filename
    videoContentType := videoHeader.Header.Get("Content-Type")

    // upload video
    _, err = storage.MinioClient.PutObject(
        context.Background(),
        storage.VideoBucketName,
        videoObjectName,
        videoFile,
        videoHeader.Size,
        minio.PutObjectOptions{ContentType: videoContentType},
    )
    if err != nil {
        http.Error(w, "Upload video to storage failed", http.StatusInternalServerError)
        return
    }

    // optional notes file (ppt/pdf)
    var notesObjectName string
    var notesHeaderSize int64
    notesFile, notesHeader, err := r.FormFile("notes")
    if err == nil {
        defer notesFile.Close()
        notesObjectName = uuid.New().String() + "-" + notesHeader.Filename
        notesContentType := notesHeader.Header.Get("Content-Type")
        notesHeaderSize = notesHeader.Size

        _, err = storage.MinioClient.PutObject(
            context.Background(),
            storage.VideoBucketName,
            notesObjectName,
            notesFile,
            notesHeaderSize,
            minio.PutObjectOptions{ContentType: notesContentType},
        )
        if err != nil {
            // If notes upload fails, you could choose to rollback video, or just log and continue
            http.Error(w, "Upload notes to storage failed", http.StatusInternalServerError)
            return
        }
    } else {
        // no notes, that's OK
        notesObjectName = ""
    }

    // Save metadata to DB, returning the video ID
    var newID int64
    if notesObjectName != "" {
        err = storage.DB.QueryRow(
            `INSERT INTO videos (title, description, filename, notes_filename)
             VALUES ($1, $2, $3, $4) RETURNING id`,
            title, description, videoObjectName, notesObjectName,
        ).Scan(&newID)
    } else {
        err = storage.DB.QueryRow(
            `INSERT INTO videos (title, description, filename)
             VALUES ($1, $2, $3) RETURNING id`,
            title, description, videoObjectName,
        ).Scan(&newID)
    }
    if err != nil {
        http.Error(w, "Database insert failed", http.StatusInternalServerError)
        return
    }

    // return detail JSON of uploaded video
    detail := VideoDetail{
        ID:            newID,
        Title:         title,
        Description:   description,
        Filename:      videoObjectName,
        URL:           fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.VideoBucketName, videoObjectName),
        NotesFilename: notesObjectName,
    }
    if notesObjectName != "" {
        detail.NotesURL = fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.VideoBucketName, notesObjectName)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(detail)
}

// VideosHandler lists video metadata (no notes in list)
func VideosHandler(w http.ResponseWriter, r *http.Request) {
    rows, err := storage.DB.Query("SELECT id, title, description, filename FROM videos ORDER BY uploaded_at DESC")
    if err != nil {
        http.Error(w, "Database query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var videos []VideoDetail
    for rows.Next() {
        var v VideoDetail
        var filename string
        if err := rows.Scan(&v.ID, &v.Title, &v.Description, &filename); err != nil {
            continue
        }
        v.Filename = filename
        v.URL = fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.VideoBucketName, filename)
        videos = append(videos, v)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(videos)
}

// VideoDetailHandler returns detail including notes, for a given video id
func VideoDetailHandler(w http.ResponseWriter, r *http.Request) {
    // Expect query param ?id=123 or path parameter; here we use query param
    idParam := r.URL.Query().Get("id")
    if idParam == "" {
        http.Error(w, "Missing id param", http.StatusBadRequest)
        return
    }
    var v VideoDetail
    var notesFilename sql.NullString  // import "database/sql"
    err := storage.DB.QueryRow(
        "SELECT id, title, description, filename, notes_filename FROM videos WHERE id = $1",
        idParam,
    ).Scan(&v.ID, &v.Title, &v.Description, &v.Filename, &notesFilename)
    if err != nil {
        http.Error(w, "Video not found", http.StatusNotFound)
        return
    }
    v.URL = fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.VideoBucketName, v.Filename)
    if notesFilename.Valid && notesFilename.String != "" {
        v.NotesFilename = notesFilename.String
        v.NotesURL = fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.VideoBucketName, notesFilename.String)
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(v)
}

// DeleteVideoHandler stays same, but if you want also to delete notes file, include that
func DeleteVideoHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodDelete {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req DeleteRequest
    err := json.NewDecoder(r.Body).Decode(&req)
    if err != nil || req.Filename == "" {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Remove video object
    err = storage.MinioClient.RemoveObject(context.Background(), storage.VideoBucketName, req.Filename, minio.RemoveObjectOptions{})
    if err != nil {
        http.Error(w, "Failed to delete video from storage", http.StatusInternalServerError)
        return
    }

    // Also find if there is notes for that video, to delete
    var notesFilename sql.NullString
    err = storage.DB.QueryRow("SELECT notes_filename FROM videos WHERE filename=$1", req.Filename).Scan(&notesFilename)
    if err == nil {
        if notesFilename.Valid && notesFilename.String != "" {
            _ = storage.MinioClient.RemoveObject(context.Background(), storage.VideoBucketName, notesFilename.String, minio.RemoveObjectOptions{})
        }
    }

    // Delete DB record
    _, err = storage.DB.Exec("DELETE FROM videos WHERE filename = $1", req.Filename)
    if err != nil {
        http.Error(w, "Failed to delete from DB", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "✅ Video deleted successfully")
}
