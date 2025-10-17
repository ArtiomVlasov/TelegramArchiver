package preview

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ArtemVlasov/TelegramArchiver/utils"
)

func Generate(inputPath, start, end, outPath string) error {
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputPath,
		"-ss", start,
		"-to", end,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-preset", "ultrafast",
		outPath,
	)
	return cmd.Run()
}

func HandlePreview(w http.ResponseWriter, r *http.Request, logger *log.Logger) {
	phrase := r.FormValue("phrase")
	url := r.FormValue("url")

	logger.Printf("New preview request: phrase=%q, url=%q", phrase, url)

	if phrase == "" {
		http.Error(w, "missing phrase", http.StatusBadRequest)
		logger.Println("Error: missing phrase")
		return
	}

	videoPath, audioPath, err := utils.SaveAndConvert(url, r, logger, "360")
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}
	defer os.Remove(videoPath)
	defer os.Remove(audioPath)

	hits, err := utils.ParseAndMatch(audioPath, phrase, logger)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}

	var outPaths []string
	for i, hit := range hits {
		parts := strings.Split(hit, "-->")
		if len(parts) != 2 {
			continue
		}
		start := strings.TrimSpace(parts[0])
		end := strings.TrimSpace(parts[1])

		outPath := filepath.Join(os.TempDir(), "preview_"+strconv.Itoa(i)+".mp4")
		if err := Generate(videoPath, start, end, outPath); err != nil {
			logger.Printf("ffmpeg error for %s: %v", hit, err)
			http.Error(w, "failed to generate preview", http.StatusInternalServerError)
			continue
		}
		outPaths = append(outPaths, outPath)
	}

	zipPath := filepath.Join(os.TempDir(), "previews.zip")
	zipFile, _ := os.Create(zipPath)
	zipWriter := zip.NewWriter(zipFile)

	for _, p := range outPaths {
		f, _ := os.Open(p)
		defer f.Close()
		defer os.Remove(p)
		wr, _ := zipWriter.Create(filepath.Base(p))
		io.Copy(wr, f)
	}
	zipWriter.Close()
	zipFile.Close()

	defer os.Remove(zipPath)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="previews.zip"`)
	http.ServeFile(w, r, zipPath)

}
