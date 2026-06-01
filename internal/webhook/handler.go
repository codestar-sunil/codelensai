package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"codelensai/internal/config"
	"codelensai/internal/github"
	"codelensai/internal/reviewer"
)

type WebhookPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title string `json:"title"`
	} `json:"pull_request"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

type Handler struct {
	config   *config.Config
	github   *github.Client
	reviewer *reviewer.Reviewer
}

func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		config:   cfg,
		github:   github.NewClient(cfg),
		reviewer: reviewer.NewReviewer(cfg),
	}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Hub-Signature-256")
	if !verifySignature(body, signature, h.config.GitHubWebhookSecret) {
		log.Println("Invalid webhook signature")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing webhook payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")

	w.WriteHeader(http.StatusOK)

	go h.processEvent(eventType, payload)
}

func (h *Handler) processEvent(eventType string, payload WebhookPayload) {
	if eventType != "pull_request" {
		log.Printf("Ignoring event type: %s", eventType)
		return
	}

	if payload.Action != "opened" && payload.Action != "synchronize" {
		log.Printf("Ignoring PR action: %s", payload.Action)
		return
	}

	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	prNumber := payload.Number
	prTitle := payload.PullRequest.Title
	installationID := payload.Installation.ID

	log.Printf("Processing PR #%d (%s) on %s/%s", prNumber, prTitle, owner, repo)

	token, err := h.github.GetInstallationToken(installationID)
	if err != nil {
		log.Printf("Error getting installation token: %v", err)
		return
	}

	diff, err := h.github.GetPRDiff(owner, repo, prNumber, token)
	if err != nil {
		log.Printf("Error fetching PR diff: %v", err)
		return
	}

	review, err := h.reviewer.Review(diff, prTitle)
	if err != nil {
		log.Printf("Error generating review: %v", err)
		failMsg := "CodeLens AI review failed, please try again"
		if postErr := h.github.PostReview(owner, repo, prNumber, failMsg, nil, token); postErr != nil {
			log.Printf("Error posting failure comment: %v", postErr)
		}
		return
	}

	var ghComments []github.ReviewComment
	for _, c := range review.Comments {
		ghComments = append(ghComments, github.ReviewComment{
			Path: c.Path,
			Line: c.Line,
			Body: c.Body,
		})
	}

	if err := h.github.PostReview(owner, repo, prNumber, review.Summary, ghComments, token); err != nil {
		log.Printf("Error posting review: %v", err)
	} else {
		log.Printf("Successfully posted review for PR #%d on %s/%s (%d inline comments)", prNumber, owner, repo, len(ghComments))
	}
}

func verifySignature(payload []byte, signature, secret string) bool {
	if secret == "" {
		return true
	}

	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}
