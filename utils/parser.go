package utils

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func Parse(audioPath, phrase string, logger *log.Logger) ([]string, *ErrorWithCode){
	cmd := exec.Command("../whisper.cpp/build/bin/whisper-cli",
		"-m", "../whisper.cpp/models/ggml-base.bin", "-t", "6",
		"-f", audioPath, "-ovtt",
		"--language", "ru",
	)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	logger.Printf("Running whisper on %s", audioPath)
	if err := cmd.Run(); err != nil {
		err := &ErrorWithCode{"whisper error: "+outBuf.String(), http.StatusInternalServerError}
		return nil, err
	}

	txtFile := strings.TrimSuffix(audioPath, ".wav") + ".wav.vtt"
	data, err := os.ReadFile(txtFile)
	if err != nil {
		err := &ErrorWithCode{"no transcription found", http.StatusInternalServerError}
		return nil, err
	}
	defer os.Remove(txtFile)

	lines := strings.Split(string(data), "\n")
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
	return hits, nil
}