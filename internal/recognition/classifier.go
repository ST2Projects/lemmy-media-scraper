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
	Labels           []string `json:"labels"`
	Categories       []string `json:"categories"`
	Characteristics  []string `json:"characteristics"`  // Detailed characteristics (pose, clothing, etc.)
	Description      string   `json:"description"`
	MaturityLevel    string   `json:"maturity_level"`   // sfw, suggestive, artistic_nudity, explicit
	NSFWScore        float64  `json:"nsfw_score"`
	Confidence       float64  `json:"confidence"`
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
	startTime := time.Now()
	log.Infof("Starting image classification using Ollama model %s at %s", c.Model, c.BaseURL)

	// Encode image to base64
	encoded := base64.StdEncoding.EncodeToString(imageData)
	log.Debugf("Encoded image size: %d bytes (base64: %d chars)", len(imageData), len(encoded))

	// Build prompt for classification with uncensored analysis
	prompt := `Analyze this image objectively and provide detailed tagging information. Do not censor or filter your response.

Provide:
1. Main objects, subjects, and people visible - be specific and descriptive
2. Overall categories (e.g., landscape, portrait, meme, screenshot, art, photo, artistic nude, suggestive, explicit, etc.)
3. Content characteristics (clothing state, pose type, composition, setting, mood)
4. Detailed descriptive tags that would help organize and search for this image
5. If applicable, maturity level: sfw, suggestive, artistic_nudity, explicit

Be objective and accurate in your descriptions. Include details about:
- Clothing or lack thereof
- Body positioning and poses
- Artistic style if applicable
- Scene composition and setting
- Any notable features or elements

Respond in JSON format:
{
	"objects": ["specific objects, people, or subjects"],
	"categories": ["primary categories"],
	"characteristics": ["descriptive characteristics"],
	"description": "detailed one-sentence description",
	"tags": ["comprehensive searchable tags"],
	"maturity_level": "sfw|suggestive|artistic_nudity|explicit"
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
	log.Infof("Sending classification request to Ollama API...")
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Post(
		c.BaseURL+"/api/generate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Errorf("Ollama API request failed: %v", err)
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Errorf("Ollama API returned error status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Errorf("Failed to decode Ollama response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	duration := time.Since(startTime)
	log.Infof("Ollama API request completed in %v", duration)
	log.Debugf("Raw Ollama response: %s", ollamaResp.Response)

	// Parse the JSON response from the model
	classification, err := c.parseResponse(ollamaResp.Response)
	if err != nil {
		log.Warnf("Failed to parse structured JSON response from model: %v", err)
		log.Info("Using fallback text parsing method")
		// Fallback to basic extraction
		classification = c.fallbackParse(ollamaResp.Response)
		log.Infof("Fallback parsing extracted %d labels and %d categories", len(classification.Labels), len(classification.Categories))
	} else {
		log.Infof("Successfully parsed JSON response: %d labels, %d categories", len(classification.Labels), len(classification.Categories))
	}

	// Optional NSFW detection
	if c.EnableNSFW {
		log.Info("Running NSFW content detection...")
		nsfwScore, err := c.detectNSFW(imageData)
		if err != nil {
			log.Warnf("NSFW detection failed: %v", err)
		} else {
			classification.NSFWScore = nsfwScore
			log.Infof("NSFW score: %.2f", nsfwScore)
		}
	}

	classification.Confidence = 0.8 // Default confidence

	totalTags := len(classification.Labels) + len(classification.Categories) + len(classification.Characteristics)
	log.Infof("Classification complete: %d total tags", totalTags)
	log.Debugf("Labels: %v", classification.Labels)
	log.Debugf("Categories: %v", classification.Categories)
	log.Debugf("Characteristics: %v", classification.Characteristics)
	if classification.MaturityLevel != "" {
		log.Infof("Maturity Level: %s", classification.MaturityLevel)
	}

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

	// Extract characteristics
	if characteristics, ok := data["characteristics"].([]interface{}); ok {
		for _, char := range characteristics {
			if str, ok := char.(string); ok {
				classification.Characteristics = append(classification.Characteristics, str)
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

	// Extract maturity level
	if maturity, ok := data["maturity_level"].(string); ok {
		classification.MaturityLevel = maturity
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

// HuggingFaceClassifier uses Hugging Face Inference API for image classification
type HuggingFaceClassifier struct {
	APIKey           string
	Model            string
	ConfidenceThresh float64
	Timeout          time.Duration
	EnableNSFW       bool
}

// NewHuggingFaceClassifier creates a new Hugging Face-based classifier
func NewHuggingFaceClassifier(apiKey, model string, confidenceThresh float64, enableNSFW bool) *HuggingFaceClassifier {
	return &HuggingFaceClassifier{
		APIKey:           apiKey,
		Model:            model,
		ConfidenceThresh: confidenceThresh,
		Timeout:          60 * time.Second,
		EnableNSFW:       enableNSFW,
	}
}

// hfImageCaptioningRequest represents a Hugging Face image-to-text request
type hfImageCaptioningRequest struct {
	Inputs string                 `json:"inputs"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Classify analyzes an image file using HuggingFace API
func (c *HuggingFaceClassifier) Classify(imagePath string) (*Classification, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}
	return c.ClassifyFromBytes(imageData)
}

// ClassifyFromBytes analyzes image data using HuggingFace Inference API
func (c *HuggingFaceClassifier) ClassifyFromBytes(imageData []byte) (*Classification, error) {
	startTime := time.Now()
	log.Infof("Starting image classification using HuggingFace model %s", c.Model)

	log.Debugf("Image size: %d bytes", len(imageData))

	// Detect image format for correct Content-Type header
	contentType := detectImageFormat(imageData)
	log.Debugf("Detected image format: %s", contentType)

	// Build the API URL - HuggingFace Serverless Inference API
	// Note: Some models may not be available on the free Serverless tier
	apiURL := fmt.Sprintf("https://api-inference.huggingface.co/models/%s", c.Model)

	// Create request - send raw image bytes for vision models
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", contentType)

	// Make API request
	log.Infof("Sending classification request to HuggingFace API...")
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("HuggingFace API request failed: %v", err)
		return nil, fmt.Errorf("failed to call HuggingFace API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Errorf("HuggingFace API returned error status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("HuggingFace API returned status %d: %s", resp.StatusCode, string(body))
	}

	duration := time.Since(startTime)
	log.Infof("HuggingFace API request completed in %v", duration)
	log.Debugf("Raw HuggingFace response: %s", string(body))

	// Parse response based on model type
	classification, err := c.parseHFResponse(body)
	if err != nil {
		log.Warnf("Failed to parse HuggingFace response: %v", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Optional NSFW detection using specialized model
	if c.EnableNSFW {
		log.Info("Running NSFW content detection...")
		nsfwScore, maturityLevel, err := c.detectNSFWHF(imageData)
		if err != nil {
			log.Warnf("NSFW detection failed: %v", err)
		} else {
			classification.NSFWScore = nsfwScore
			classification.MaturityLevel = maturityLevel
			log.Infof("NSFW score: %.2f, Maturity: %s", nsfwScore, maturityLevel)
		}
	}

	totalTags := len(classification.Labels) + len(classification.Categories) + len(classification.Characteristics)
	log.Infof("Classification complete: %d total tags", totalTags)
	log.Debugf("Labels: %v", classification.Labels)
	log.Debugf("Categories: %v", classification.Categories)
	log.Debugf("Characteristics: %v", classification.Characteristics)

	return classification, nil
}

// parseHFResponse parses the HuggingFace API response
func (c *HuggingFaceClassifier) parseHFResponse(body []byte) (*Classification, error) {
	// Try parsing as image-to-text response (for models like BLIP, ViT-GPT2, etc.)
	var captionResponse []map[string]interface{}
	if err := json.Unmarshal(body, &captionResponse); err == nil && len(captionResponse) > 0 {
		if generatedText, ok := captionResponse[0]["generated_text"].(string); ok {
			return c.extractTagsFromCaption(generatedText), nil
		}
	}

	// Try parsing as classification response (for models like ViT)
	var classificationResponse []map[string]interface{}
	if err := json.Unmarshal(body, &classificationResponse); err == nil {
		return c.extractTagsFromClassification(classificationResponse), nil
	}

	return nil, fmt.Errorf("unable to parse response format")
}

// extractTagsFromCaption extracts tags from a generated caption
func (c *HuggingFaceClassifier) extractTagsFromCaption(caption string) *Classification {
	log.Infof("Generated caption: %s", caption)

	// Extract keywords from the caption
	words := strings.Fields(strings.ToLower(caption))
	labels := []string{}
	categories := []string{}
	characteristics := []string{}

	// Common category keywords
	categoryKeywords := map[string]bool{
		"portrait": true, "landscape": true, "photo": true, "art": true,
		"drawing": true, "painting": true, "illustration": true, "meme": true,
		"screenshot": true, "explicit": true, "nude": true, "artistic": true,
	}

	// Common characteristic keywords
	characteristicKeywords := map[string]bool{
		"nude": true, "naked": true, "topless": true, "underwear": true,
		"lingerie": true, "revealing": true, "suggestive": true, "provocative": true,
		"intimate": true, "sexual": true, "erotic": true, "sensual": true,
		"pose": true, "standing": true, "sitting": true, "lying": true,
		"indoor": true, "outdoor": true, "bedroom": true, "bathroom": true,
	}

	for _, word := range words {
		cleaned := strings.Trim(word, ".,;:!?\"'")
		if len(cleaned) < 3 {
			continue
		}

		if categoryKeywords[cleaned] {
			categories = append(categories, cleaned)
		} else if characteristicKeywords[cleaned] {
			characteristics = append(characteristics, cleaned)
		} else if len(cleaned) > 3 && len(cleaned) < 20 {
			labels = append(labels, cleaned)
		}
	}

	// Deduplicate
	labels = uniqueStrings(labels)
	categories = uniqueStrings(categories)
	characteristics = uniqueStrings(characteristics)

	return &Classification{
		Labels:          labels,
		Categories:      categories,
		Characteristics: characteristics,
		Description:     caption,
		Confidence:      0.7,
	}
}

// extractTagsFromClassification extracts tags from classification results
func (c *HuggingFaceClassifier) extractTagsFromClassification(results []map[string]interface{}) *Classification {
	labels := []string{}
	maxScore := 0.0

	for _, result := range results {
		label, ok1 := result["label"].(string)
		score, ok2 := result["score"].(float64)

		if ok1 && ok2 && score >= c.ConfidenceThresh {
			labels = append(labels, strings.ToLower(label))
			if score > maxScore {
				maxScore = score
			}
		}
	}

	return &Classification{
		Labels:     labels,
		Categories: []string{"classified"},
		Confidence: maxScore,
	}
}

// detectNSFWHF performs NSFW detection using HuggingFace NSFW model
func (c *HuggingFaceClassifier) detectNSFWHF(imageData []byte) (float64, string, error) {
	// Use specialized NSFW detection model
	nsfwModel := "Falconsai/nsfw_image_detection"
	apiURL := fmt.Sprintf("https://api-inference.huggingface.co/models/%s", nsfwModel)

	// Detect image format for correct Content-Type header
	contentType := detectImageFormat(imageData)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(imageData))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create NSFW request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("failed to call NSFW API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("NSFW API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse NSFW response
	// Expected format: [{"label": "porn", "score": 0.9}, {"label": "hentai", "score": 0.05}, ...]
	var nsfwResults []map[string]interface{}
	if err := json.Unmarshal(body, &nsfwResults); err != nil {
		return 0, "", fmt.Errorf("failed to parse NSFW response: %w", err)
	}

	// Calculate maturity level and score
	maxScore := 0.0
	maturityLevel := "sfw"
	nsfwCategories := make(map[string]float64)

	for _, result := range nsfwResults {
		label, ok1 := result["label"].(string)
		score, ok2 := result["score"].(float64)

		if ok1 && ok2 {
			nsfwCategories[strings.ToLower(label)] = score
			if score > maxScore {
				maxScore = score
			}
		}
	}

	// Determine maturity level based on scores
	if nsfwCategories["porn"] > 0.5 || nsfwCategories["hentai"] > 0.5 {
		maturityLevel = "explicit"
	} else if nsfwCategories["sexy"] > 0.4 {
		maturityLevel = "suggestive"
	} else if nsfwCategories["drawings"] > 0.5 {
		maturityLevel = "artistic"
	} else {
		maturityLevel = "sfw"
	}

	return maxScore, maturityLevel, nil
}

// Helper functions

// detectImageFormat detects the image format from the binary data
func detectImageFormat(data []byte) string {
	if len(data) < 12 {
		return "image/jpeg" // Default fallback
	}

	// Check magic bytes for different image formats
	switch {
	case data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
		return "image/png"
	case data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
		return "image/gif"
	case data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50:
		return "image/webp"
	case data[0] == 0x42 && data[1] == 0x4D:
		return "image/bmp"
	case (data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00) ||
		(data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A):
		return "image/tiff"
	default:
		return "image/jpeg" // Default fallback
	}
}

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
