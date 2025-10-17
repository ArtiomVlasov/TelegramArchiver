package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"time"

	"github.com/ArtemVlasov/TelegramArchiver/services/preview"
	"github.com/ArtemVlasov/TelegramArchiver/services/search"
)




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
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {search.HandleSearch(w, r, logger)})
	http.HandleFunc("/preview", func(w http.ResponseWriter, r *http.Request) {preview.HandlePreview(w, r, logger)})
	logger.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}




