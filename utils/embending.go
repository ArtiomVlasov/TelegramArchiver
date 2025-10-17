package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type Sentence struct {
	Text      string    `json:"text"`
	Start     float64   `json:"start"`
	End       float64   `json:"end"`
	Embedding []float64 `json:"embedding,omitempty"`
}

type jinaRequest struct {
	Model string   `json:"model"`
	Task  string   `json:"task"`
	Input []string `json:"input"`
}

type jinaResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func GenerateEmbeddings(sentences []Sentence, logger *log.Logger) ([]Sentence, error) {
	url := "https://api.jina.ai/v1/embeddings"

	if err := godotenv.Load(".env"); err != nil {
		logger.Printf("Error loading .env: %v", err)
		return nil, &ErrorWithCode{"Error enabling .env file: " + err.Error(), http.StatusInternalServerError}
	}

	apiKey := os.Getenv("JINA_API_KEY")
	if apiKey == "" {
		logger.Println("JINA_API_KEY not found in .env")
		return nil, &ErrorWithCode{"Server error", http.StatusInternalServerError}
	}
	logger.Println(apiKey)

	inputs := make([]string, len(sentences))
	for i, s := range sentences {
		inputs[i] = s.Text
	}

	logger.Println(inputs)

	bodyData := jinaRequest{
		Model: "jina-embeddings-v3",
		Task:  "text-matching",
		Input: inputs,
	}

	body, err := json.Marshal(bodyData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Jina API: %w", err)
	}
	fmt.Println(resp.Body)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Парсим ответ
	var result jinaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if len(result.Data) != len(sentences) {
		return nil, fmt.Errorf("unexpected embeddings count: got %d, want %d",
			len(result.Data), len(sentences))
	}

	for i := range sentences {
		sentences[i].Embedding = result.Data[i].Embedding
	}

	return sentences, nil
}
