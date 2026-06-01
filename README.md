# CodeLens AI - Automated PR Code Review

CodeLens AI is a GitHub App that automatically reviews Pull Requests using Claude AI. When a PR is opened or updated, it fetches the diff, sends it to Claude for analysis, and posts the review as a comment on the PR.

## Features

- Automatic code review on PR open and update
- Security vulnerability detection
- Performance issue identification
- Error handling analysis
- Best practice recommendations
- Webhook signature verification for security

## Prerequisites

- Go 1.21+
- A GitHub App (see setup below)
- An Anthropic Claude API key

## Setup

### 1. Create a GitHub App

1. Go to **GitHub Settings > Developer settings > GitHub Apps > New GitHub App**
2. Fill in the details:
   - **GitHub App name**: CodeLens AI (or your preferred name)
   - **Homepage URL**: `https://github.com/your-username/codelensai`
   - **Webhook URL**: Your server URL + `/webhook/github` (e.g., `https://webhooktunnel.onrender.com/t/codelensai/webhook/github` for local dev)
   - **Webhook secret**: Generate a strong secret and save it
3. Set **Permissions**:
   - **Pull requests**: Read and Write
   - **Contents**: Read
4. **Subscribe to events**:
   - Pull request
5. Click **Create GitHub App**
6. Note the **App ID** from the app settings page
7. Scroll down and click **Generate a private key** — download the `.pem` file

### 2. Configure Environment Variables

Copy the example env file:

```bash
cp .env.example .env
```

Fill in your `.env` file:

```
PORT=3000
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
GITHUB_WEBHOOK_SECRET=your-webhook-secret
CLAUDE_API_KEY=sk-ant-...
```

> **Note**: For `GITHUB_PRIVATE_KEY`, paste the contents of your `.pem` file. Replace newlines with `\n` or use multi-line format supported by your shell.

### 3. Install Dependencies

```bash
go mod tidy
```

### 4. Run the Server

```bash
go run main.go
```

You should see:

```
CodeLens AI server running on port 3000
```

### 5. Local Development with WebhookTunnel

To test locally, use WebhookTunnel to expose your local server:

```bash
webhooktunnel start --port 3000 --name codelensai
```

Set your GitHub App webhook URL to:

```
https://webhooktunnel.onrender.com/t/codelensai/webhook/github
```

### 6. Install the App

1. Go to your GitHub App settings
2. Click **Install App** in the sidebar
3. Select the repositories you want CodeLens AI to review
4. Open a PR on one of those repos — the review will appear as a comment

## How It Works

1. A developer opens or updates a Pull Request
2. GitHub sends a webhook event to the server
3. The server verifies the webhook signature
4. It fetches the PR diff via the GitHub API
5. The diff is sent to Claude AI for review
6. Claude's review is posted back as a PR review comment

## Project Structure

```
codelensai/
├── main.go                        # Entry point, HTTP server setup
├── go.mod                         # Go module definition
├── .env.example                   # Example environment variables
└── internal/
    ├── config/
    │   └── config.go              # Environment variable loading
    ├── webhook/
    │   └── handler.go             # Webhook receipt and signature verification
    ├── github/
    │   └── client.go              # GitHub API client (diff, reviews, tokens)
    └── reviewer/
        └── reviewer.go            # Claude AI review integration
```
