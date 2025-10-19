package findthemes

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"media-analyzer/utils"
)

type SegmentResponse struct {
	TopicID int     `json:"topic_id"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
}

func cosineSim(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
func meanVector(sents []utils.Sentence) []float64 {
	if len(sents) == 0 {
		return nil
	}
	vec := make([]float64, len(sents[0].Embedding))
	for _, s := range sents {
		for i, v := range s.Embedding {
			vec[i] += v
		}
	}
	for i := range vec {
		vec[i] /= float64(len(sents))
	}
	return vec
}

func SegmentText(sentences []utils.Sentence, windowSize int, threshold float64) [][]utils.Sentence {
	if len(sentences) == 0 {
		return nil
	}

	simScores := make([]float64, len(sentences)-1)
	for i := 0; i < len(sentences)-1; i++ {
		startA := max(0, i-windowSize)
		endA := min(len(sentences), i)
		startB := i + 1
		endB := min(len(sentences), i+1+windowSize)

		vecA := meanVector(sentences[startA:endA])
		vecB := meanVector(sentences[startB:endB])
		simScores[i] = cosineSim(vecA, vecB)
	}

	var segments [][]utils.Sentence
	current := []utils.Sentence{sentences[0]}
	for i := 1; i < len(sentences); i++ {
		if i-1 < len(simScores) && simScores[i-1] < threshold {
			segments = append(segments, current)
			current = []utils.Sentence{}
		}
		current = append(current, sentences[i])
	}
	if len(current) > 0 {
		segments = append(segments, current)
	}

	return segments
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func adaptiveParams(sentences []utils.Sentence) (int, float64) {
	N := len(sentences)
	if N == 0 {
		return 3, 0.7
	}

	wordSet := make(map[string]struct{})
	var totalWords int
	for _, s := range sentences {
		words := strings.Fields(strings.ToLower(s.Text))
		totalWords += len(words)
		for _, w := range words {
			wordSet[w] = struct{}{}
		}
	}
	lexicalVariety := float64(len(wordSet)) / float64(totalWords)
	if lexicalVariety > 1 {
		lexicalVariety = 1
	}

	windowSize := int(0.05 * float64(N))
	if windowSize < 3 {
		windowSize = 3
	} else if windowSize > 10 {
		windowSize = 10
	}

	threshold := 0.7 + 0.1*(1-lexicalVariety)
	if threshold > 0.85 {
		threshold = 0.85
	} else if threshold < 0.55 {
		threshold = 0.55
	}

	return windowSize, threshold
}

func HandleThemes(w http.ResponseWriter, r *http.Request, logger *log.Logger) {
	start := time.Now()
	url := r.FormValue("url")

	logger.Printf("New request: url=%q", url)

	videoPath, audioPath, err := utils.SaveAndConvert(url, r, logger, "144")
	if videoPath != "" {
		defer os.Remove(videoPath)
	}
	if audioPath != "" {
		defer os.Remove(audioPath)
	}

	if err != nil {
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}

	sentences, err := utils.Parse(audioPath, logger)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}

	sentences, err = utils.GenerateEmbeddings(sentences, logger)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, err.Error(), err.Code)
		return
	}

	windowSize, threshold := adaptiveParams(sentences)
	logger.Printf("Adaptive params: window=%d threshold=%.2f", windowSize, threshold)
	segments := SegmentText(sentences, windowSize, threshold)

	var response []SegmentResponse
	for topicID, group := range segments {
		if len(group) == 0 {
			continue
		}

		startTime := group[0].Start
		endTime := group[len(group)-1].End

		var textBuilder strings.Builder
		for _, s := range group {
			textBuilder.WriteString(s.Text + " ")
		}

		response = append(response, SegmentResponse{
			TopicID: topicID + 1,
			Start:   startTime,
			End:     endTime,
			Text:    strings.TrimSpace(textBuilder.String()),
		})
	}

	minDuration := 15.0 // секунд
	filtered := []SegmentResponse{}
	for _, seg := range response {
		if seg.End-seg.Start < minDuration && len(filtered) > 0 {
			// Склеиваем с предыдущим
			prev := &filtered[len(filtered)-1]
			prev.End = seg.End
			prev.Text += " " + seg.Text
		} else {
			filtered = append(filtered, seg)
		}
	}
	response = filtered

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	logger.Printf("Finished processing in %s", time.Since(start))
}
