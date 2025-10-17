package utils

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofrs/uuid"
)

type ErrorWithCode struct {
	Message string
	Code    int
}

func (e *ErrorWithCode) Error() string {
	return fmt.Sprintf("error : %s", e.Message)
}

func SaveAndConvert(url string, r *http.Request, logger *log.Logger, quality string) (string, string, *ErrorWithCode) {
	id := uuid.Must(uuid.NewV4()).String()
	var videoPath string
	if url != "" {
		id := uuid.Must(uuid.NewV4()).String()
		basePath := filepath.Join(os.TempDir(), id)

		format := fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best", quality)

		cmdName := exec.Command(
			"yt-dlp",
			"-f", format,
			"-o", basePath+".%(ext)s",
			"--print", "filename",
			url,
		)

		output, err := cmdName.Output()
		if err != nil {
			logger.Printf("yt-dlp print error: %v", err)
			return "", "", &ErrorWithCode{"failed to determine filename", 500}
		}

		videoPath = strings.TrimSpace(string(output))
		cmd := exec.Command(
			"yt-dlp",
			"-f", format,
			"-o", videoPath,
			url,
		)

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out	
		logger.Printf("Running yt-dlp for %s", url)
		if err := cmd.Run(); err != nil {
			logger.Printf("yt-dlp error: %v\nOutput:\n%s", err, out.String())
			err := &ErrorWithCode{"yt-dlp error: " + out.String(), http.StatusInternalServerError}
			return videoPath, "", err
		}
	} else {
		file, header, err := r.FormFile("file")
		if err != nil {
			err := &ErrorWithCode{err.Error() + "missing file or url", http.StatusBadRequest}
			logger.Println("Error: missing file or url")
			return "", "", err
		}

		videoPath = filepath.Join(os.TempDir(), id+"_"+header.Filename)
		out, err := os.Create(videoPath)
		if err != nil {
			err := &ErrorWithCode{err.Error(), http.StatusInternalServerError}
			logger.Printf("File create error: %v", err)
			return "", "", err
		}
		defer out.Close()
		io.Copy(out, file)
		logger.Printf("Received uploaded file: %s", header.Filename)
	}
	logger.Println(videoPath)
	audioPath := filepath.Join(os.TempDir(), id+".wav")
	logger.Printf("Converting to WAV: %s → %s", videoPath, audioPath)
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ar", "16000", "-ac", "1", "-f", "wav", audioPath)
	if err := cmd.Run(); err != nil {
		logger.Printf("ffmpeg error: %v", err)
		err := &ErrorWithCode{"ffmpeg error: " + err.Error(), http.StatusInternalServerError}
		return videoPath, audioPath, err
	}

	return videoPath, audioPath, nil
}
