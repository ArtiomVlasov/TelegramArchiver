package preview

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ArtemVlasov/TelegramArchiver/utils"
)

type PreviewResult struct {
	Timestamp string `json:"timestamp"`
	Preview   string `json:"preview"`
}

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

	hits, err := utils.Parse(audioPath, phrase, logger)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}

	var results []PreviewResult
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

		results = append(results, PreviewResult{
			Timestamp: hit,
			Preview:   "/static/" + filepath.Base(outPath),
		})

		outPaths = append(outPaths, outPath)
	}

	// for _, p := range outPaths {
	// 	defer os.Remove(p)
	// }

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", `attachment; filename="preview.mp4"`)
	http.ServeFile(w, r, outPaths[0])

}
