package handlers

import (
    "encoding/json"
    "net/http"
    "os"
    "os/exec"
	"fmt"

)

type CodeRequest struct {
    Language string `json:"language"` // "python", "c", "go"
    Code     string `json:"code"`
}

type CodeResponse struct {
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}

func RunHandler(w http.ResponseWriter, r *http.Request) {
    var req CodeRequest

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    var (
        sourcePath string
        outputPath string
        cmd        *exec.Cmd
    )

    // Create a working dir if it doesn't exist
    _ = os.MkdirAll("./code", 0755)

    switch req.Language {
    case "python":
        sourcePath = "./code/code.py"
        err := os.WriteFile(sourcePath, []byte(req.Code), 0644)
        if err != nil {
            respondWithError(w, "Failed to write Python code: "+err.Error())
            return
        }
        cmd = exec.Command("python3", sourcePath)

    case "c":
        sourcePath = "./code/code.c"
        outputPath = "./code/code.out"

        err := os.WriteFile(sourcePath, []byte(req.Code), 0644)
        if err != nil {
            respondWithError(w, "Failed to write C code: "+err.Error())
            return
        }

        // Compile the C code
        compile := exec.Command("gcc", sourcePath, "-o", outputPath)
        compileOutput, err := compile.CombinedOutput()
        if err != nil {
            respondWithOutput(w, string(compileOutput), err.Error())
            return
        }

        // Run the compiled program
        cmd = exec.Command(outputPath)

    case "go":
        sourcePath = "./code/code.go"
        err := os.WriteFile(sourcePath, []byte(req.Code), 0644)
        if err != nil {
            respondWithError(w, "Failed to write Go code: "+err.Error())
            return
        }

        cmd = exec.Command("go", "run", sourcePath)

    default:
        respondWithError(w, "Unsupported language: "+req.Language)
        return
    }

    // Run the command and capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        respondWithOutput(w, string(output), err.Error())
        return
    }

    respondWithOutput(w, string(output), "")
}

// Utility functions
func respondWithError(w http.ResponseWriter, msg string) {
    respondWithOutput(w, "", msg)
}

func respondWithOutput(w http.ResponseWriter, output string, errMsg string) {
    response := CodeResponse{
        Output: output,
        Error:  errMsg,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Optional: serve HTML editor UI
func ServeCodeRunner(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "templates/code_editor.html")
}

func serveForm(w http.ResponseWriter,r *http.Request) {
     http.ServeFile(w,r,"templates/cs.html")
}

func main() {
    http.HandleFunc("/run", RunHandler)
    http.HandleFunc("/code", ServeCodeRunner)
    http.HandleFunc("/form",serveForm)

    fmt.Println("🚀 Server running at http://localhost:8080")
    err := http.ListenAndServe(":8080", nil) // Start the server
    if err != nil {
        fmt.Println("Failed to start server:", err)
    }
}