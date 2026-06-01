package github

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"codelensai/internal/config"

	"github.com/golang-jwt/jwt/v4"
)

type Client struct {
	config *config.Config
	http   *http.Client
}

type ReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

type ReviewRequest struct {
	Body     string          `json:"body"`
	Event    string          `json:"event"`
	Comments []ReviewComment `json:"comments,omitempty"`
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) GetInstallationToken(installationID int64) (string, error) {
	jwtToken, err := c.generateJWT()
	if err != nil {
		return "", fmt.Errorf("generating JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	return result.Token, nil
}

func (c *Client) GetPRDiff(owner, repo string, prNumber int, token string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.diff")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading diff body: %w", err)
	}

	diff := string(body)
	if len(diff) > 10000 {
		log.Printf("Diff truncated from %d to 10000 characters", len(diff))
		diff = diff[:10000]
	}

	return diff, nil
}

func (c *Client) PostReview(owner, repo string, prNumber int, summary string, comments []ReviewComment, token string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)

	payload := ReviewRequest{
		Body:     summary,
		Event:    "COMMENT",
		Comments: comments,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling review body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("posting review: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) generateJWT() (string, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(c.config.GitHubPrivateKey))
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	appID, err := strconv.ParseInt(c.config.GitHubAppID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parsing app ID: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    strconv.FormatInt(appID, 10),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString((*rsa.PrivateKey)(privateKey))
}
