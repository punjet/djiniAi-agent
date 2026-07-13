# =============================================================================
# Build stage — compile the Go binary
# =============================================================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app-binary ./cmd/career-ops

# =============================================================================
# Runtime stage
#   Needs Chromium for chromedp PDF generation (Go native, no Node.js).
# =============================================================================
FROM alpine:3.20

# ---- Chromium + system deps for chromedp headless PDF generation ----
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    font-noto \
    wget

WORKDIR /app

# ---- Go binary ----
COPY --from=builder /app/app-binary .

# ---- Career-ops context directory ----
# Contains cv.md, config/profile.yml, modes/*.md, templates/*.html,
# fonts/ — the pipeline reads all of these at runtime for CV/cover-letter
# generation.
COPY --from=builder /app/career-ops ./career-ops

# ---- Runtime directories the pipeline expects ----
RUN mkdir -p /app/career-ops/output \
             /app/career-ops/logs \
             /app/career-ops/batch/tracker-additions \
             /app/career-ops/reports

# =============================================================================
# Default entrypoint
# =============================================================================
# Runs the autonomous scan-evaluate-apply pipeline in daemon mode using OpenAI.
# The container sends interactive Telegram messages (inline keyboards) when a
# high-scoring job is found, and waits for your ✅ / ✍️ / ❌ decision.
#
# Required environment variables at runtime:
#
#   LLM (OpenAI)
#     OPENAI_API_KEY          — your OpenAI API key
#
#   Djinni authentication (for scanning and applying)
#     DJINNI_SESSIONID         — session cookie from djinni.co
#     DJINNI_CSRFTOKEN         — csrf token cookie from djinni.co
#
#   Telegram bot (for human-in-the-loop approvals)
#     TG_BOT_TOKEN             — Telegram bot token from @BotFather
#     TG_CHAT_ID               — your Telegram chat/user ID
#
# Optional:
#     FREELLMAPI_MODEL         — OpenAI model name (default: gpt-4o-mini)
#     --threshold              — score threshold (default: 3.5, set via args)
#     --dry-run                — scan-only, no real submissions (add to CMD)
#
# Example run:
#   docker run -d \
#     --name djinni-bot \
#     -e OPENAI_API_KEY="sk-..." \
#     -e DJINNI_SESSIONID="..." \
#     -e DJINNI_CSRFTOKEN="..." \
#     -e TG_BOT_TOKEN="..." \
#     -e TG_CHAT_ID="..." \
#     djinni-bot
# =============================================================================
ENTRYPOINT ["./app-binary"]
CMD ["pipeline", "run", "--engine", "openai", "--daemon"]
