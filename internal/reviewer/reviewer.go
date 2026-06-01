package reviewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
- "summary" should include an overall assessment and mention what was done well
- If there are no inline issues, return an empty "comments" array
- Only comment on lines that exist in the new side of the diff

PR Title: %s

Diff:
%s`, prTitle, diff)

	reqBody := claudeRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 2000,
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

	var review ReviewResult
	if err := json.Unmarshal([]byte(text), &review); err != nil {
		// Fallback: if Claude didn't return valid JSON, use raw text as summary
		return &ReviewResult{
			Summary:  text,
			Comments: nil,
		}, nil
	}

	return &review, nil
}
