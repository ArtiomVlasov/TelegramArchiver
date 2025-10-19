package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

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

func GenerateEmbeddings(sentences []Sentence, logger *log.Logger) ([]Sentence, *ErrorWithCode) {
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

	inputs := make([]string, len(sentences))
	for i, s := range sentences {
		inputs[i] = s.Text
	}

	bodyData := jinaRequest{
		Model: "jina-embeddings-v3",
		Task:  "text-matching",
		Input: inputs,
	}

	body, err := json.Marshal(bodyData)
	if err != nil {
		return nil, &ErrorWithCode{"failed to marshal request: " + err.Error(), http.StatusInternalServerError}
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil,  &ErrorWithCode{"failed to create HTTP request: " + err.Error(), http.StatusInternalServerError}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &ErrorWithCode{"failed to call Jina API: " + err.Error(), http.StatusInternalServerError}
	}
	logger.Println(resp)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrorWithCode{"failed to read response body: " + err.Error(), http.StatusInternalServerError}
	}
	var result jinaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &ErrorWithCode{"failed to decode response: " + err.Error(), http.StatusInternalServerError}
	}
	logger.Println(result)
	logger.Println(resp)

	if len(result.Data) != len(sentences) {
		return nil, &ErrorWithCode{"unexpected embeddings count: got"+ strconv.Itoa(len(result.Data)) +", want "+ strconv.Itoa(len(sentences)), http.StatusInternalServerError}
	}

	for i := range sentences {
		sentences[i].Embedding = result.Data[i].Embedding
	}
	return sentences, nil
}
