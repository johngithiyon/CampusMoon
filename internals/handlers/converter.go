package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const UploadDir = "./uploads"

func ConvertToTextHandler(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse the form to get the fileUrl
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	fileURL := r.FormValue("fileUrl")
	if fileURL == "" {
		http.Error(w, "fileUrl is required", http.StatusBadRequest)
		return
	}

	log.Printf("Processing file: %s", fileURL)

	// ---------------- Download if it’s a URL ----------------
	var localFilePath string
	if strings.HasPrefix(fileURL, "http") {
		resp, err := http.Get(fileURL)
		if err != nil {
			http.Error(w, "Failed to download audio file", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Remove query params from filename
		baseName := filepath.Base(strings.Split(fileURL, "?")[0])
		localFilePath = filepath.Join(UploadDir, baseName)

		// Ensure uploads folder exists
		os.MkdirAll(UploadDir, os.ModePerm)

		out, err := os.Create(localFilePath)
		if err != nil {
			http.Error(w, "Failed to create local file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			http.Error(w, "Failed to save audio file", http.StatusInternalServerError)
			return
		}
	} else {
		// Assume it's a local path like /uploads/audio.mp3
		localFilePath = strings.TrimPrefix(fileURL, "/")
	}

	// ---------------- Run Whisper ----------------
	inputDir := filepath.Dir(localFilePath)
	baseName := strings.TrimSuffix(filepath.Base(localFilePath), filepath.Ext(localFilePath))
	txtFile := filepath.Join(inputDir, baseName+".txt")

	log.Printf("Executing: python3 -m whisper %s --model base --output_format txt --output_dir %s", localFilePath, inputDir)
	cmd := exec.Command(
		"python3", "-m", "whisper",
		localFilePath,
		"--model", "base",
		"--output_format", "txt",
		"--output_dir", inputDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Whisper error: %v, Output: %s", err, string(output))
		http.Error(w, fmt.Sprintf("Transcription failed: %v, Output: %s", err, string(output)), http.StatusInternalServerError)
		return
	}

	// ---------------- Read Transcription ----------------
	text, err := os.ReadFile(txtFile)
	if err != nil {
		log.Printf("Error reading text file: %v", err)
		http.Error(w, "Error reading transcribed text", http.StatusInternalServerError)
		return
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"text": string(text)})
}
