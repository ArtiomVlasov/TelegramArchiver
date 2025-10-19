package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	listen "github.com/deepgram/deepgram-go-sdk/pkg/api/listen/v1/rest"
	interfaces "github.com/deepgram/deepgram-go-sdk/pkg/client/interfaces"
	client "github.com/deepgram/deepgram-go-sdk/pkg/client/listen"
	"github.com/joho/godotenv"
)


type DeepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Paragraphs struct {
					Paragraphs []struct {
						Sentences []Sentence `json:"sentences"`
					} `json:"paragraphs"`
				} `json:"paragraphs"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

func Parse(audioPath string, logger *log.Logger) ([]Sentence, *ErrorWithCode) {
	if err := godotenv.Load(".env"); err != nil {
		logger.Printf("Error loading .env: %v", err)
		return nil, &ErrorWithCode{"Error enabling .env file: " + err.Error(), http.StatusInternalServerError}
	}

	apiKey := os.Getenv("DEEPGRAM_API_KEY")
	if apiKey == "" {
		logger.Println("DEEPGRAM_API_KEY not found in .env")
		return nil, &ErrorWithCode{"Server error", http.StatusInternalServerError}
	}

	ctx := context.Background()
	c := client.NewREST(apiKey, &interfaces.ClientOptions{})
	dg := listen.New(c)

	options := &interfaces.PreRecordedTranscriptionOptions{
		Model:       "nova-2",
		SmartFormat: true,
		Language:    "ru",
		Punctuate:   true,
	}

	res, err := dg.FromFile(ctx, audioPath, options)
	if err != nil {
		return nil, &ErrorWithCode{fmt.Sprintf("Deepgram API error: %v", err), http.StatusInternalServerError}
	}

	raw, err := json.Marshal(res)
	if err != nil {
		return nil, &ErrorWithCode{"failed to marshal Deepgram response", http.StatusInternalServerError}
	}

	var dgResp DeepgramResponse
	if err := json.Unmarshal(raw, &dgResp); err != nil {
		return nil, &ErrorWithCode{"failed to parse Deepgram JSON", http.StatusInternalServerError}
	}

	var sentences []Sentence
	for _, ch := range dgResp.Results.Channels {
		for _, alt := range ch.Alternatives {
			for _, p := range alt.Paragraphs.Paragraphs {
				sentences = append(sentences, p.Sentences...)
			}
		}
	}

	if len(sentences) == 0 {
		return nil, &ErrorWithCode{"no sentences found", http.StatusNotFound}
	}
	return sentences, nil
}

func ParseAndMatch(audioPath, phrase string, logger *log.Logger) ([]string, *ErrorWithCode) {
	sentences, err := Parse(audioPath, logger)
	if err != nil {
		return nil, err
	}

	var hits []string
	for _, s := range sentences {
		if strings.Contains(strings.ToLower(s.Text), strings.ToLower(phrase)) {
			hits = append(hits, fmt.Sprintf("%.2f --> %.2f", s.Start, s.End))
		}
	}

	if len(hits) == 0 {
		return nil, &ErrorWithCode{"no matches found", http.StatusNotFound}
	}

	return hits, nil
}