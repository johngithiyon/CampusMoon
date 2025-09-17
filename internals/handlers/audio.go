package handlers

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/gorilla/mux"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    _ "github.com/lib/pq" // or your database driver
)

// Audio represents an audio file with metadata
type Audio struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Filename    string    `json:"filename"`
    URL         string    `json:"url"`
    SubjectID   int       `json:"subjectId"`
    UploadedAt  time.Time `json:"uploadedAt"`
}

var (
    MinioClient *minio.Client
    audioBucket = "audios"
    DB          *sql.DB // Add database connection
)

// InitAudioHandlers initializes MinIO, database connection and ensures bucket exists
func InitAudioHandlers(db *sql.DB) {
    // Store database connection
    DB = db

    endpoint := os.Getenv("MINIO_ENDPOINT")
    accessKey := os.Getenv("MINIO_ACCESS_KEY")
    secretKey := os.Getenv("MINIO_SECRET_KEY")
    useSSL := false

    var err error
    MinioClient, err = minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        log.Fatalf("Failed to initialize MinIO client: %v", err)
    }

    ctx := context.Background()
    err = MinioClient.MakeBucket(ctx, audioBucket, minio.MakeBucketOptions{})
    if err != nil {
        exists, errBucketExists := MinioClient.BucketExists(ctx, audioBucket)
        if errBucketExists != nil || !exists {
            log.Fatalf("Failed to create bucket: %v", err)
        }
    }
    
    // Create table if it doesn't exist
    createAudioTable()
    
    log.Printf("Connected to MinIO and ensured bucket '%s' exists", audioBucket)
}

// createAudioTable creates the audio_files table if it doesn't exist
func createAudioTable() {
    query := `
    CREATE TABLE IF NOT EXISTS audio_files (
        id VARCHAR(255) PRIMARY KEY,
        title VARCHAR(255) NOT NULL,
        description TEXT,
        filename VARCHAR(255) NOT NULL,
        url VARCHAR(512) NOT NULL,
        subject_id INTEGER NOT NULL,
        uploaded_at TIMESTAMP NOT NULL
    );
    
    CREATE INDEX IF NOT EXISTS idx_audio_subject_id ON audio_files(subject_id);
    `
    
    if _, err := DB.Exec(query); err != nil {
        log.Fatalf("Failed to create audio_files table: %v", err)
    }
}

// RegisterAudioRoutes registers audio-related routes on the provided router
func RegisterAudioRoutes(r *mux.Router) {
    r.HandleFunc("/upload-audio", HandleAudioUpload).Methods("POST")
    r.HandleFunc("/audio", HandleGetAudio).Methods("GET")
    r.HandleFunc("/delete-audio", HandleDeleteAudio).Methods("DELETE")
    r.HandleFunc("/audio/{filename}", HandleAudioStream).Methods("GET")
}

// ------------------- HANDLERS -------------------

func HandleAudioUpload(w http.ResponseWriter, r *http.Request) {
    err := r.ParseMultipartForm(32 << 20)
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    title := r.FormValue("title")
    description := r.FormValue("description")
    subjectIDStr := r.FormValue("subjectId")

    if title == "" || subjectIDStr == "" {
        http.Error(w, "Title and subject ID are required", http.StatusBadRequest)
        return
    }

    subjectID, err := strconv.Atoi(subjectIDStr)
    if err != nil {
        http.Error(w, "Invalid subject ID", http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("audio")
    if err != nil {
        http.Error(w, "Failed to get audio file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    ext := filepath.Ext(header.Filename)
    filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

    ctx := context.Background()
    contentType := header.Header.Get("Content-Type")
    _, err = MinioClient.PutObject(ctx, audioBucket, filename, file, header.Size, minio.PutObjectOptions{
        ContentType: contentType,
    })
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to upload file: %v", err), http.StatusInternalServerError)
        return
    }

    fileURL := fmt.Sprintf("http://%s/%s/%s", os.Getenv("MINIO_ENDPOINT"), audioBucket, filename)
    audioID := saveAudioMetadata(title, description, filename, fileURL, subjectID)

    audio := Audio{
        ID:          audioID,
        Title:       title,
        Description: description,
        Filename:    filename,
        URL:         fileURL,
        SubjectID:   subjectID,
        UploadedAt:  time.Now(),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(audio)
}

func HandleGetAudio(w http.ResponseWriter, r *http.Request) {
    subjectIDStr := r.URL.Query().Get("subjectId")
    if subjectIDStr == "" {
        http.Error(w, "Subject ID is required", http.StatusBadRequest)
        return
    }
    subjectID, err := strconv.Atoi(subjectIDStr)
    if err != nil {
        http.Error(w, "Invalid subject ID", http.StatusBadRequest)
        return
    }

    audios, err := getAudioFilesBySubject(subjectID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to get audio files: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(audios)
}

func HandleDeleteAudio(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Filename  string `json:"filename"`
        SubjectID int    `json:"subjectId"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Failed to parse request body", http.StatusBadRequest)
        return
    }

    if req.Filename == "" {
        http.Error(w, "Filename is required", http.StatusBadRequest)
        return
    }

    ctx := context.Background()
    err := MinioClient.RemoveObject(ctx, audioBucket, req.Filename, minio.RemoveObjectOptions{})
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to delete file: %v", err), http.StatusInternalServerError)
        return
    }

    if err := deleteAudioMetadata(req.Filename, req.SubjectID); err != nil {
        http.Error(w, fmt.Sprintf("Failed to delete metadata: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"message": "Audio deleted successfully"})
}

func HandleAudioStream(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    filename := vars["filename"]
    if filename == "" {
        http.Error(w, "Filename is required", http.StatusBadRequest)
        return
    }

    ctx := context.Background()
    object, err := MinioClient.GetObject(ctx, audioBucket, filename, minio.GetObjectOptions{})
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to get audio file: %v", err), http.StatusNotFound)
        return
    }
    defer object.Close()

    stat, err := object.Stat()
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to get file info: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", stat.ContentType)
    w.Header().Set("Content-Length", strconv.FormatInt(stat.Size, 10))

    if r.Header.Get("Range") != "" {
        ranges, err := parseRangeHeader(r.Header.Get("Range"), stat.Size)
        if err != nil {
            http.Error(w, "Invalid range header", http.StatusRequestedRangeNotSatisfiable)
            return
        }
        w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", ranges[0].start, ranges[0].end, stat.Size))
        w.WriteHeader(http.StatusPartialContent)

        _, err = object.Seek(ranges[0].start, io.SeekStart)
        if err != nil {
            http.Error(w, fmt.Sprintf("Failed to seek in file: %v", err), http.StatusInternalServerError)
            return
        }

        _, err = io.CopyN(w, object, ranges[0].end-ranges[0].start+1)
        if err != nil && err != io.EOF {
            http.Error(w, fmt.Sprintf("Failed to stream file: %v", err), http.StatusInternalServerError)
            return
        }
    } else {
        _, err = io.Copy(w, object)
        if err != nil {
            http.Error(w, fmt.Sprintf("Failed to stream file: %v", err), http.StatusInternalServerError)
            return
        }
    }
}

// ------------------- HELPERS -------------------

func saveAudioMetadata(title, description, filename, url string, subjectID int) string {
    id := fmt.Sprintf("audio-%d", time.Now().UnixNano())
    
    query := `
    INSERT INTO audio_files (id, title, description, filename, url, subject_id, uploaded_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
    
    _, err := DB.Exec(query, id, title, description, filename, url, subjectID, time.Now())
    if err != nil {
        log.Printf("Failed to save audio metadata: %v", err)
        return ""
    }
    
    return id
}

func getAudioFilesBySubject(subjectID int) ([]Audio, error) {
    query := `
    SELECT id, title, description, filename, url, subject_id, uploaded_at
    FROM audio_files
    WHERE subject_id = $1
    ORDER BY uploaded_at DESC
    `
    
    rows, err := DB.Query(query, subjectID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var audios []Audio
    for rows.Next() {
        var audio Audio
        err := rows.Scan(
            &audio.ID,
            &audio.Title,
            &audio.Description,
            &audio.Filename,
            &audio.URL,
            &audio.SubjectID,
            &audio.UploadedAt,
        )
        if err != nil {
            return nil, err
        }
        audios = append(audios, audio)
    }
    
    if err = rows.Err(); err != nil {
        return nil, err
    }
    
    return audios, nil
}

func deleteAudioMetadata(filename string, subjectID int) error {
    query := `
    DELETE FROM audio_files
    WHERE filename = $1 AND subject_id = $2
    `
    
    _, err := DB.Exec(query, filename, subjectID)
    if err != nil {
        return err
    }
    
    return nil
}

type Range struct{ start, end int64 }

func parseRangeHeader(rangeHeader string, size int64) ([]Range, error) {
    if !strings.HasPrefix(rangeHeader, "bytes=") {
        return nil, fmt.Errorf("invalid range header")
    }
    parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid range header")
    }
    start, err := strconv.ParseInt(parts[0], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid start")
    }
    end, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        end = size - 1
    }
    if start > end || start >= size || end >= size {
        return nil, fmt.Errorf("range not satisfiable")
    }
    return []Range{{start: start, end: end}}, nil
}