# Djinni AI Agent

AI-powered job search automation: scans Djinni, evaluates vacancies via LLM, generates cover letters and CVs, and applies automatically.

> **Development process & branching strategy** → see [CONTRIBUTING.md](CONTRIBUTING.md)

## Branches

| Branch | Purpose |
|--------|---------|
| `main` | Production (Coolify deploys automatically on push) |
| `develop` | Staging (integration branch, merge features here first) |
| `feature/*` | New features, branched from `develop` |
| `fix/*` | Bug fixes, branched from `develop` |

## Local Setup

### Option 1: Docker Compose (Recommended)

Run the entire application stack including PostgreSQL with pgvector, Grafana, and Loki using Docker Compose.

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Edit .env with your Djinni cookies, API keys, and database settings

# 3. Build and start all services
docker compose up -d --build
```

Access local services:
- **Application Server**: `http://localhost:8080` (or `PORT` configured in `.env`)
- **PostgreSQL**: `localhost:5432`
- **Grafana Dashboard**: `http://localhost:3000` (Default credentials: `admin` / `${GRAFANA_ADMIN_PASSWORD:-admin}`)
- **Loki Log Collector**: `http://localhost:3100`

### Option 2: Local Go Execution

If running the backend directly using Go:

1. Ensure Go 1.24 or 1.26+ is installed.
2. Make sure PostgreSQL is running locally or via Docker (`docker compose up -d db`).
3. Prepare configuration:
   ```bash
   cp .env.example .env
   ```
4. Run commands or pipeline via `make` or standard Go tools:
   ```bash
   # Run all unit tests
   make test

   # Run application pipeline
   go run ./cmd/career-ops server
   ```

## Coolify Deployment Guide

Follow these steps to deploy `djini-ai-agent`, PostgreSQL, Grafana, and Loki on a self-hosted Coolify instance.

### Step 1: Create a PostgreSQL Database Resource
1. In your Coolify Project, click **+ New Resource** → **Database** → **PostgreSQL**.
2. Set resource name to `djinni-db`.
3. Set database name (`DB_NAME`), user (`DB_USER`), and password (`DB_PASSWORD`).
4. Enable pgvector extension support if needed by your schema migrations.
5. Save destination details and start the database. Note the internal service name or IP for `DB_HOST` (e.g. `djinni-db` or internal Docker IP).

### Step 2: Deploy Grafana and Loki Services
1. Click **+ New Resource** → **Service** → **Grafana** (or create custom Docker Compose / Docker Image applications).
2. For Loki, create a service using image `grafana/loki:3.0.0` listening on port `3100`.
3. For Grafana, create a service using image `grafana/grafana:latest` on port `3000`.
4. In Grafana, configure Loki as a default HTTP datasource pointing to your Loki service endpoint (e.g., `http://loki:3100`).

### Step 3: Create and Configure `djini-ai-agent` Application
1. In Coolify, click **+ New Resource** → **Public Repository** or **GitHub App**.
2. Select repository `djiniAi-agent` and branch `main`.
3. Select **Dockerfile** as the build pack (or use `docker-compose.yml`).
4. Set expose port to `8080` (or your preferred application port).
5. Add all required environment variables under the application **Environment Variables** tab.

### Required Environment Variables Matrix

| Variable | Description | Default / Example | Required |
|----------|-------------|-------------------|----------|
| `PORT` | HTTP server listening port | `8080` | Optional |
| `DB_HOST` | PostgreSQL host | `djinni-db` | Yes |
| `DB_PORT` | PostgreSQL port | `5432` | Yes |
| `DB_USER` | PostgreSQL user | `postgres` | Yes |
| `DB_PASSWORD` | PostgreSQL password | `your-secure-db-password` | Yes |
| `DB_NAME` | PostgreSQL database name | `djinni` | Yes |
| `DJINNI_SESSIONID` | Djinni authentication session cookie | `your_sessionid_cookie` | Yes |
| `DJINNI_CSRFTOKEN` | Djinni CSRF token cookie | `your_csrftoken_cookie` | Yes |
| `GEMINI_API_KEY` | Google Gemini API key | `AIzaSy...` | Optional (if using LLM) |
| `GEMINI_MODEL` | Gemini model name | `gemini-2.5-flash` | Optional |
| `OPENAI_API_KEY` | OpenAI API key | `sk-...` | Optional (if using LLM) |
| `OPENAI_MODEL` | OpenAI model identifier | `gpt-5-mini` | Optional |
| `EXA_API_KEY` | Exa web search API key | `exa-...` | Optional |
| `LOKI_URL` | Loki push endpoint for structured logs | `http://loki:3100` | Optional |
| `TG_BOT_TOKEN` | Telegram Bot API token for notifications | `123456:ABC...` | Optional |
| `TG_CHAT_ID` | Telegram Chat ID for notifications | `-1001234567` | Optional |
| `PROFILE_PATH` | Override default path for `profile.yml` | `/app/config/profile.yml` | Optional |
| `CV_PATH` | Override default path for `cv.md` | `/app/cv.md` | Optional |

## CV & Candidate Profile Configuration Guide

To prevent committing sensitive personal data, resume details, and custom candidate preferences to git, store them using Coolify Persistent Storage files or environment mounts.

### Mounting Files in Coolify (Recommended)

1. Open the `djini-ai-agent` application settings in your Coolify dashboard.
2. Navigate to **Storages**.
3. Add a persistent file mount for candidate profile:
   - **Source/Type**: File
   - **Destination Path**: `/app/config/profile.yml` (or `/app/career-ops/config/profile.yml`)
   - Paste your private YAML profile content into the file editor.
4. Add a persistent file mount for candidate CV:
   - **Source/Type**: File
   - **Destination Path**: `/app/cv.md` (or `/app/career-ops/cv.md`)
   - Paste your Markdown CV text into the file editor.
5. Save changes and redeploy the application.

### Path Customization via Environment Variables

If you choose a custom location for your persistent mounts, update your application environment variables accordingly:

```env
PROFILE_PATH=/app/custom-mounts/profile.yml
CV_PATH=/app/custom-mounts/cv.md
```

## How to Obtain Djinni Authentication Cookies

1. Log in to [djinni.co](https://djinni.co) in your browser.
2. Open Browser Developer Tools (`F12` or right click → **Inspect**).
3. Go to the **Application** tab (Chrome/Edge) or **Storage** tab (Firefox).
4. Select **Cookies** → `https://djinni.co`.
5. Locate the `sessionid` and `csrftoken` cookie values.
6. Copy these values into your environment configuration (`DJINNI_SESSIONID` and `DJINNI_CSRFTOKEN`).

## Usage / Makefile Targets

We use `make` to streamline common tasks.

- **Build**: Compiles the `career-ops` binary.
  ```bash
  make build
  ```

- **Test**: Runs all Go tests.
  ```bash
  make test
  ```

- **Lint**: Runs standard `go vet` to catch potential issues.
  ```bash
  make lint
  ```

- **Run Evaluate**: Runs the evaluation command directly. Pass the job description text via the `JD` variable.
  ```bash
  make run-evaluate JD="We are looking for a Senior Go Engineer with 5+ years of experience..."
  ```

- **Run Test Apply**: Runs the `pipeline test-apply` diagnostic tool. This command performs an end-to-end test of the full pipeline in dry-run mode.
  ```bash
  make run-test-apply
  ```

## Observability (Grafana + Loki & MCP Setup)

The stack includes Grafana and Loki in `docker-compose.yml` for log aggregation and AI agent observability via the Grafana MCP server.

### Running Grafana + Loki

To start the observability services alongside Postgres and App:
```bash
docker compose up -d loki grafana
```
- **Grafana URL**: `http://localhost:3000` (Default credentials: `admin` / `${GRAFANA_ADMIN_PASSWORD:-admin}`)
- **Loki URL**: `http://localhost:3100` (Pre-configured as default datasource in Grafana)

### Connecting Grafana MCP Server for AI Agent Access

To enable AI agents (e.g. Claude Desktop, OpenCode, Codex) to inspect logs and query metrics via Grafana MCP server (`@grafana/mcp` or `grafana-mcp`), configure your MCP client settings (e.g., `opencode.json`, `claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "grafana": {
      "command": "npx",
      "args": ["-y", "@grafana/mcp-server"],
      "env": {
        "GRAFANA_URL": "http://localhost:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "<your-grafana-service-account-token>"
      }
    }
  }
}
```

*Steps to generate Service Account Token:*
1. Open `http://localhost:3000` in browser.
2. Go to **Administration** → **Users & access** → **Service accounts**.
3. Create a Service Account (e.g., `ai-agent`) with `Viewer` or `Editor` role.
4. Click **Add service account token**, generate token, and copy it into `GRAFANA_SERVICE_ACCOUNT_TOKEN`.

## Applied-jobs tracker

The bot maintains a persistent registry of jobs that have been applied to, stored in `data/applied_jobs.json`. This registry prevents duplicate evaluations and applications by:

1. **Registry skip check**: Before fetching job details, the bot loads the registry and skips any job whose ID or slug is already recorded.
2. **HTML-based detection**: After fetching job details, the bot checks for the HTML snippet indicating an already-applied status. If detected, the job is skipped and its ID is added to the registry.
3. **Automatic persistence**: After a successful application submission (including retry completions), the job ID is automatically saved to the registry.

The registry is a simple JSON file where keys are job IDs (numeric strings) and values are RFC3339 timestamps of the application moment. The file is written atomically to avoid corruption.
