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
    // Get parameters from query string
    id := r.URL.Query().Get("id")
    filename := r.URL.Query().Get("filename")
    
    // Get title and description from query parameters if available
    title := r.URL.Query().Get("title")
    description := r.URL.Query().Get("description")
    
    if id == "" && filename == "" {
        http.Error(w, "Missing video identifier", http.StatusBadRequest)
        return
    }

    var dbTitle, dbDescription, dbFilename string
    var notesFilename sql.NullString
    var err error

    // Query by ID if provided, otherwise by filename
    if id != "" {
        err = storage.DB.QueryRow(`
            SELECT title, description, filename, notes_filename
            FROM videos
            WHERE id = $1
        `, id).Scan(&dbTitle, &dbDescription, &dbFilename, &notesFilename)
    } else {
        err = storage.DB.QueryRow(`
            SELECT title, description, filename, notes_filename
            FROM videos
            WHERE filename = $1
        `, filename).Scan(&dbTitle, &dbDescription, &dbFilename, &notesFilename)
    }

    if err != nil {
        if err == sql.ErrNoRows {
            http.NotFound(w, r)
        } else {
            http.Error(w, "Database error", http.StatusInternalServerError)
        }
        return
    }

    // Use title and description from query parameters if provided, otherwise use DB values
    if title == "" {
        title = dbTitle
    }
    if description == "" {
        description = dbDescription
    }
    
    // If filename was not provided in the query, use the one from DB
    if filename == "" {
        filename = dbFilename
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