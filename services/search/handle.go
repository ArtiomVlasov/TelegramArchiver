package search

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ArtemVlasov/TelegramArchiver/utils"
)

type SearchResponse struct {
	Phrase string   `json:"phrase"`
	Hits   []string `json:"hits"`
}




func HandleSearch(w http.ResponseWriter, r *http.Request, logger *log.Logger) {
	start := time.Now()
	phrase := r.FormValue("phrase")
	url := r.FormValue("url")

	logger.Printf("New request: phrase=%q, url=%q", phrase, url)

	if phrase == "" {
		http.Error(w, "missing phrase", http.StatusBadRequest)
		logger.Println("Error: missing phrase")
		return
	}

	videoPath, audioPath, err := utils.SaveAndConvert(url, r, logger, "144")
	defer os.Remove(videoPath)
	defer os.Remove(audioPath)

	if err != nil{
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return 
	}

	hits, err := utils.Parse(audioPath, phrase, logger)

	if err != nil{
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}

	resp := SearchResponse{Phrase: phrase, Hits: hits}
	logger.Printf("Finished processing in %s, hits: %d", time.Since(start), len(hits))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}