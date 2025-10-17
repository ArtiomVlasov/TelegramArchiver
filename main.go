package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
)

type SearchResponse struct {
	Phrase string   `json:"phrase"`
	Hits   []string `json:"hits"`
}

var (
	logger *log.Logger
)

func init() {
	logFile := "/tmp/archiver.log"
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("cannot open log file: %v", err)
	}
	logger = log.New(io.MultiWriter(f, os.Stdout), "", log.LstdFlags|log.Lshortfile)
	logger.Println("Logging initialized at", time.Now())
}

func main() {
	http.HandleFunc("/search", handleSearch)
	logger.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	phrase := r.FormValue("phrase")
	url := r.FormValue("url")

	logger.Printf("New request: phrase=%q, url=%q", phrase, url)

	if phrase == "" {
		http.Error(w, "missing phrase", http.StatusBadRequest)
		logger.Println("Error: missing phrase")
		return
	}

	id := uuid.Must(uuid.NewV4()).String()
	var videoPath string

	if url != "" {
		videoPath = filepath.Join(os.TempDir(), id+".mp4")
		cmd := exec.Command(
			"yt-dlp",
			"-f", "worstvideo[height<=144]+worstaudio/bestaudio",
			"-o", videoPath,
			url,
		)
		videoPath += ".mkv"
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		logger.Printf("Running yt-dlp for %s", url)
		if err := cmd.Run(); err != nil {
			logger.Printf("yt-dlp error: %v\nOutput:\n%s", err, out.String())
			http.Error(w, "yt-dlp error: "+out.String(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(videoPath)
	} else {
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file or url", http.StatusBadRequest)
			logger.Println("Error: missing file or url")
			return
		}
		defer file.Close()

		videoPath = filepath.Join(os.TempDir(), id+"_"+header.Filename)
		out, err := os.Create(videoPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logger.Printf("File create error: %v", err)
			return
		}
		defer os.Remove(videoPath)
		defer out.Close()
		io.Copy(out, file)
		logger.Printf("Received uploaded file: %s", header.Filename)
	}

	audioPath := filepath.Join(os.TempDir(), id+".wav")
	logger.Printf("Converting to WAV: %s → %s", videoPath, audioPath)
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ar", "16000", "-ac", "1", "-f", "wav", audioPath)
	if err := cmd.Run(); err != nil {
		logger.Printf("ffmpeg error: %v", err)
		http.Error(w, "ffmpeg error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(audioPath)

	cmd = exec.Command("../whisper.cpp/build/bin/whisper-cli",
		"-m", "../whisper.cpp/models/ggml-base.bin", "-t", "6",
		"-f", audioPath, "-ovtt",
		"--language", "ru",
	)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	logger.Printf("Running whisper on %s", audioPath)
	if err := cmd.Run(); err != nil {
		logger.Printf("whisper error: %v\nOutput:\n%s", err, outBuf.String())
		http.Error(w, "whisper error: "+outBuf.String(), http.StatusInternalServerError)
		return
	}

	txtFile := strings.TrimSuffix(audioPath, ".wav") + ".wav.vtt"
	data, err := os.ReadFile(txtFile)
	if err != nil {
		logger.Printf("No transcription found for %s", audioPath)
		http.Error(w, "no transcription found", http.StatusInternalServerError)
		return
	}
	defer os.Remove(txtFile)

	fmt.Println(phrase)
	lines := strings.Split(string(data), "\n")
	logger.Println(lines)
	var hits []string

	for i := 0; i < len(lines)-1; i++ {
		line := strings.TrimSpace(lines[i])
		next := strings.TrimSpace(lines[i+1])

		if strings.Contains(line, "-->") && i+1 < len(lines) {
			if strings.Contains(strings.ToLower(next), strings.ToLower(phrase)) {
				hits = append(hits, line) 
			}
		}
	}

	resp := SearchResponse{Phrase: phrase, Hits: hits}
	logger.Printf("Finished processing %s in %s, hits: %d", id, time.Since(start), len(hits))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
