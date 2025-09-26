package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConvertVideoToNotesHandler handles video-to-text conversion
func ConvertVideoToNotesHandler(w http.ResponseWriter, r *http.Request) {
	// CORS headers
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

	// Parse request
	var req struct {
		VideoUrl   string `json:"videoUrl"`
		VideoTitle string `json:"videoTitle"`
		Download   bool   `json:"download"` // optional: if true, serve as file
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error parsing request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "video-transcript-*")
	if err != nil {
		http.Error(w, "Error creating temp directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Download video
	videoPath := filepath.Join(tempDir, "video.mp4")
	if err := downloadFile(req.VideoUrl, videoPath); err != nil {
		http.Error(w, "Error downloading video: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract audio
	audioPath := filepath.Join(tempDir, "audio.wav")
	if err := extractAudio(videoPath, audioPath); err != nil {
		http.Error(w, "Error extracting audio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Transcribe audio
	transcript, err := audioToText(audioPath)
	if err != nil {
		http.Error(w, "Error converting audio to text: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Serve as downloadable text file if requested
	if req.Download {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.txt\"", sanitizeFileName(req.VideoTitle)))
		io.WriteString(w, transcript)
		return
	}

	// Otherwise return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"notes": transcript,
	})
}

// Helper: sanitize file names
func sanitizeFileName(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		default:
			return r
		}
	}, name)
}

// downloadFile downloads file from URL
func downloadFile(url, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractAudio extracts audio using ffmpeg
func extractAudio(videoPath, audioPath string) error {
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vn", "-acodec", "pcm_s16le", "-ar", "16000", "-ac", "1", audioPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, Output: %s", err, string(output))
	}
	return nil
}

// audioToText transcribes audio using Whisper
func audioToText(audioPath string) (string, error) {
	outputDir := filepath.Dir(audioPath)
	cmd := exec.Command(
		"python3", "-m", "whisper",
		audioPath,
		"--model", "base",
		"--output_format", "txt",
		"--output_dir", outputDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper error: %v, Output: %s", err, string(output))
	}

	baseName := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	txtFile := filepath.Join(outputDir, baseName+".txt")
	text, err := os.ReadFile(txtFile)
	if err != nil {
		return "", fmt.Errorf("error reading transcript: %v", err)
	}
	return string(text), nil
}
