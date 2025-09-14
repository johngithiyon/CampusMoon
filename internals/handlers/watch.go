package handlers

import (
    "CampusMoon/internals/storage"
    "database/sql"
    "fmt"
    "html/template"
    "net/http"
)

type VideoPageData struct {
    Title       string
    Description string
    VideoURL    string
    NotesURL    string
}

var tmpl = template.Must(template.ParseFiles("templates/watch.html"))

func VideoPageHandler(w http.ResponseWriter, r *http.Request) {
    // Get video ID from query string
    id := r.URL.Query().Get("id")
    if id == "" {
        http.Error(w, "Missing video ID", http.StatusBadRequest)
        return
    }

    // Get video info from DB
    var title, description, filename string
    var notesFilename sql.NullString
    err := storage.DB.QueryRow(`
        SELECT title, description, filename, notes_filename
        FROM videos
        WHERE id = $1
    `, id).Scan(&title, &description, &filename, &notesFilename)

    if err != nil {
        if err == sql.ErrNoRows {
            http.NotFound(w, r)
        } else {
            http.Error(w, "Database error", http.StatusInternalServerError)
        }
        return
    }

    // Construct full URLs
    videoURL := fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.BucketName, filename)
    var notesURL string
    if notesFilename.Valid && notesFilename.String != "" {
        notesURL = fmt.Sprintf("%s/%s/%s", PublicURLPrefix, storage.BucketName, notesFilename.String)
    }

    data := VideoPageData{
        Title:       title,
        Description: description,
        VideoURL:    videoURL,
        NotesURL:    notesURL,
    }

    // Render template
    err = tmpl.Execute(w, data)
    if err != nil {
        http.Error(w, "Template render error", http.StatusInternalServerError)
    }
}
