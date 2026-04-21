package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/HimanshuSardana/capsule/pkg/container"
)

type RunRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Stdin    string `json:"stdin"`
}

type RunResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func main() {
	http.HandleFunc("/run", corsMiddleware(handleRun))
	http.HandleFunc("/health", corsMiddleware(handleHealth))

	fmt.Println("Starting API server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	}
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := runCode(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func runCode(req RunRequest) (*RunResponse, error) {
	rand.Seed(time.Now().UnixNano())
	subID := rand.Intn(1000000)
	subName := fmt.Sprintf("submission-%d", subID)

	baseDir := "/tmp/capsule/submissions"
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir base: %w", err)
	}

	subDir := filepath.Join(baseDir, subName)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir subdir: %w", err)
	}

	imgPath := "images/base-python.tar.gz"
	if err := extractImage(imgPath, subDir); err != nil {
		os.RemoveAll(subDir)
		return nil, fmt.Errorf("extract: %w", err)
	}

	codeFile := fmt.Sprintf("%d.py", subID)
	codePath := filepath.Join(subDir, codeFile)
	if err := os.WriteFile(codePath, []byte(req.Code), 0o644); err != nil {
		os.RemoveAll(subDir)
		return nil, fmt.Errorf("write code: %w", err)
	}

	stdinFile := filepath.Join(subDir, "stdin.txt")
	if req.Stdin != "" {
		if err := os.WriteFile(stdinFile, []byte(req.Stdin), 0o644); err != nil {
			os.RemoveAll(subDir)
			return nil, fmt.Errorf("write stdin: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] subDir=%s codeFile=%s stdinFile=%s\n", subDir, codeFile, stdinFile)

	stdinData, _ := os.ReadFile(stdinFile)
	fmt.Fprintf(os.Stderr, "[DEBUG] stdin content: %q\n", string(stdinData))

	exitCode, stdout, stderr := container.ExecuteInSandbox(subDir, codeFile, stdinFile)
	fmt.Fprintf(os.Stderr, "[DEBUG] exitCode=%d stdout=%q stderr=%q\n", exitCode, stdout, stderr)

	os.RemoveAll(subDir)

	return &RunResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}, nil
}

func extractImage(imgPath, dest string) error {
	cmd := exec.Command("tar", "-xzf", imgPath, "-C", dest)
	return cmd.Run()
}