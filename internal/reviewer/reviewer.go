package reviewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"codelensai/internal/config"
)

type Reviewer struct {
	config *config.Config
	http   *http.Client
}

type InlineComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

type ReviewResult struct {
	Summary  string          `json:"summary"`
	Comments []InlineComment `json:"comments"`
}

func NewReviewer(cfg *config.Config) *Reviewer {
	return &Reviewer{
		config: cfg,
		http:   &http.Client{Timeout: 120 * time.Second},
	}
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (r *Reviewer) Review(diff, prTitle string) (*ReviewResult, error) {
	prompt := fmt.Sprintf(`You are a senior software engineer reviewing a pull request.

Analyze this diff and provide feedback on:
- Security vulnerabilities
- Performance issues
- Missing error handling
- Bad practices
- What was done well

IMPORTANT: You must respond with ONLY valid JSON (no markdown fences, no extra text).
Use this exact JSON structure:

{
  "summary": "Overall review summary in markdown format",
  "comments": [
    {
      "path": "relative/file/path.go",
      "line": 42,
      "body": "Description of the issue and how to fix it"
    }
  ]
}

Rules for the JSON response:
- "path" must be the file path exactly as shown in the diff header (after b/)
- "line" must be a line number from the NEW side of the diff (lines starting with + in the diff, using the line number shown after the @@ hunk header)
- "body" should be clear, specific, and actionable markdown
- "summary" should be a brief overall assessment (3-5 sentences max) mentioning key concerns and what was done well
- Put detailed feedback in the "comments" array, NOT in the summary
- If there are no inline issues, return an empty "comments" array
- Only comment on lines that exist in the new side of the diff
- Prioritize the most important issues (max 10 comments)

PR Title: %s

Diff:
%s`, prTitle, diff)

	reqBody := claudeRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 4096,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", r.config.ClaudeAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Claude API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Claude API error (%d): %s", resp.StatusCode, string(body))
	}

	var result claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	text := result.Content[0].Text

	// Extract JSON from response - find the outermost { }
	jsonStr := extractJSON(text)

	var review ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &review); err != nil {
		log.Printf("Failed to parse review JSON: %v", err)
		log.Printf("Extracted JSON: %.500s", jsonStr)
		// Fallback: post raw text as summary
		return &ReviewResult{
			Summary:  text,
			Comments: nil,
		}, nil
	}

	log.Printf("Parsed review: %d comments, summary length: %d", len(review.Comments), len(review.Summary))
	return &review, nil
}

func extractJSON(text string) string {
	// Find first { and last }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		return text[start : end+1]
	}
	return text
}
