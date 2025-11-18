package recognition

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Classification represents the result of image analysis
type Classification struct {
	Labels      []string `json:"labels"`
	Categories  []string `json:"categories"`
	Description string   `json:"description"`
	NSFWScore   float64  `json:"nsfw_score"`
	Confidence  float64  `json:"confidence"`
}

// Classifier interface for image recognition
type Classifier interface {
	Classify(imagePath string) (*Classification, error)
	ClassifyFromBytes(imageData []byte) (*Classification, error)
}

// OllamaClassifier uses Ollama API for image classification
type OllamaClassifier struct {
	BaseURL           string
	Model             string
	ConfidenceThresh  float64
	Timeout           time.Duration
	EnableNSFW        bool
}

// NewOllamaClassifier creates a new Ollama-based classifier
func NewOllamaClassifier(baseURL, model string, confidenceThresh float64, enableNSFW bool) *OllamaClassifier {
	return &OllamaClassifier{
		BaseURL:          strings.TrimSuffix(baseURL, "/"),
		Model:            model,
		ConfidenceThresh: confidenceThresh,
		Timeout:          60 * time.Second,
		EnableNSFW:       enableNSFW,
	}
}

// ollamaRequest represents an Ollama API request
type ollamaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
	Stream bool     `json:"stream"`
}

// ollamaResponse represents an Ollama API response
type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// Classify analyzes an image file
func (c *OllamaClassifier) Classify(imagePath string) (*Classification, error) {
	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	return c.ClassifyFromBytes(imageData)
}

// ClassifyFromBytes analyzes image data in memory
func (c *OllamaClassifier) ClassifyFromBytes(imageData []byte) (*Classification, error) {
	// Encode image to base64
	encoded := base64.StdEncoding.EncodeToString(imageData)

	// Build prompt for classification
	prompt := `Analyze this image and provide:
1. Main objects and subjects visible (comma-separated list)
2. Overall categories (e.g., landscape, portrait, meme, screenshot, art, photo)
3. Brief description (one sentence)
4. Tags that would help organize this image

Respond in JSON format:
{
	"objects": ["object1", "object2"],
	"categories": ["category1", "category2"],
	"description": "brief description",
	"tags": ["tag1", "tag2"]
}`

	// Prepare request
	reqData := ollamaRequest{
		Model:  c.Model,
		Prompt: prompt,
		Images: []string{encoded},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API request
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Post(
		c.BaseURL+"/api/generate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse the JSON response from the model
	classification, err := c.parseResponse(ollamaResp.Response)
	if err != nil {
		log.Debugf("Failed to parse structured response, using fallback: %v", err)
		// Fallback to basic extraction
		classification = c.fallbackParse(ollamaResp.Response)
	}

	// Optional NSFW detection
	if c.EnableNSFW {
		nsfwScore, err := c.detectNSFW(imageData)
		if err != nil {
			log.Debugf("NSFW detection failed: %v", err)
		} else {
			classification.NSFWScore = nsfwScore
		}
	}

	classification.Confidence = 0.8 // Default confidence

	log.Debugf("Image classification: %+v", classification)

	return classification, nil
}

// parseResponse extracts classification data from Ollama's response
func (c *OllamaClassifier) parseResponse(response string) (*Classification, error) {
	// Try to find JSON in the response
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")

	if startIdx == -1 || endIdx == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[startIdx : endIdx+1]

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	classification := &Classification{}

	// Extract objects/labels
	if objects, ok := data["objects"].([]interface{}); ok {
		for _, obj := range objects {
			if str, ok := obj.(string); ok {
				classification.Labels = append(classification.Labels, str)
			}
		}
	}

	// Extract categories
	if categories, ok := data["categories"].([]interface{}); ok {
		for _, cat := range categories {
			if str, ok := cat.(string); ok {
				classification.Categories = append(classification.Categories, str)
			}
		}
	}

	// Extract tags (add to labels)
	if tags, ok := data["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if str, ok := tag.(string); ok {
				classification.Labels = append(classification.Labels, str)
			}
		}
	}

	// Extract description
	if desc, ok := data["description"].(string); ok {
		classification.Description = desc
	}

	return classification, nil
}

// fallbackParse provides basic extraction when structured parsing fails
func (c *OllamaClassifier) fallbackParse(response string) *Classification {
	// Extract potential tags/labels from the response
	words := strings.Fields(response)
	var labels []string

	// Simple heuristic: look for capitalized words and common objects
	for _, word := range words {
		cleaned := strings.Trim(word, ".,;:!?\"'")
		if len(cleaned) > 2 && len(cleaned) < 20 {
			// Add if it looks like a label
			if isLikelyLabel(cleaned) {
				labels = append(labels, strings.ToLower(cleaned))
			}
		}
	}

	// Deduplicate
	labels = uniqueStrings(labels)

	return &Classification{
		Labels:      labels,
		Categories:  []string{"general"},
		Description: truncate(response, 200),
		Confidence:  0.5, // Lower confidence for fallback
	}
}

// detectNSFW performs NSFW content detection
func (c *OllamaClassifier) detectNSFW(imageData []byte) (float64, error) {
	// Encode image to base64
	encoded := base64.StdEncoding.EncodeToString(imageData)

	prompt := `Is this image safe for work (SFW) or not safe for work (NSFW)?
Rate the NSFW content on a scale of 0.0 (completely safe) to 1.0 (explicit content).
Respond with only a number between 0.0 and 1.0.`

	reqData := ollamaRequest{
		Model:  c.Model,
		Prompt: prompt,
		Images: []string{encoded},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Post(
		c.BaseURL+"/api/generate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Try to extract a number from the response
	var score float64
	_, err = fmt.Sscanf(ollamaResp.Response, "%f", &score)
	if err != nil {
		// Fallback: look for keywords
		lower := strings.ToLower(ollamaResp.Response)
		if strings.Contains(lower, "nsfw") || strings.Contains(lower, "explicit") {
			return 0.9, nil
		}
		if strings.Contains(lower, "sfw") || strings.Contains(lower, "safe") {
			return 0.1, nil
		}
		return 0.5, nil // Uncertain
	}

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score, nil
}

// Helper functions

func isLikelyLabel(word string) bool {
	// Common objects, simple heuristic
	commonWords := map[string]bool{
		"photo": true, "image": true, "picture": true,
		"landscape": true, "portrait": true, "nature": true,
		"person": true, "people": true, "animal": true,
		"building": true, "sky": true, "water": true,
		"tree": true, "flower": true, "car": true,
		"food": true, "art": true, "meme": true,
		"screenshot": true, "text": true, "diagram": true,
	}

	lower := strings.ToLower(word)
	return commonWords[lower] || (len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z')
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, val := range slice {
		if !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
