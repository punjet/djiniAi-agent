# Djinni AI Agent

AI-powered job search automation: scans Djinni, evaluates vacancies via LLM, generates cover letters and CVs, and applies automatically.

> **Development process & branching strategy** → see [CONTRIBUTING.md](CONTRIBUTING.md)

## Branches

| Branch | Purpose |
|--------|---------|
| `main` | Production — Coolify deploys automatically on push |
| `develop` | Staging — integration branch, merge features here first |
| `feature/*` | New features, branched from `develop` |
| `fix/*` | Bug fixes, branched from `develop` |

## Prerequisites

- **Go**: Version 1.24 or 1.26 compatible.
- **Ollama** (optional): For running local LLM evaluation (`llama3.1:8b` by default).

## Setup

1. Navigate to the `djinni-bot-go` directory:
   ```bash
   cd djinni-bot-go
   ```

2. Copy the `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

3. Fill out the `.env` variables:

   **Djinni Authentication**
   *   `DJINNI_SESSIONID`: Your Djinni session ID cookie.
   *   `DJINNI_CSRFTOKEN`: Your Djinni CSRF token cookie.
   
   *How to get cookies:*
   1. Log into [djinni.co](https://djinni.co).
   2. Open your browser's Developer Tools (F12 or right-click -> Inspect).
   3. Go to the **Application** tab (Chrome/Edge) or **Storage** tab (Firefox).
   4. Expand **Cookies** and select `https://djinni.co`.
   5. Find the rows named `sessionid` and `csrftoken`.
   6. Copy their Values and paste them into your `.env` file.

   **Gemini API Configuration**
   *   `GEMINI_API_KEY`: Your Google Gemini API key.
   *   `GEMINI_MODEL`: (Default: `gemini-2.0-flash-exp`)

   **Ollama Configuration**
   *   `OLLAMA_MODEL`: (Default: `llama3.1:8b`)
   *   `OLLAMA_BASE_URL`: (Default: `http://localhost:11434`)

   **FreeLLMAPI Configuration**
   *   `FREELLMAPI_BASE_URL`: (Default: `https://free.llmapi.com`)

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
  *Note: Evaluating job descriptions only requires LLM keys; it does not require Djinni cookies unless full Djinni operations are being invoked.*

- **Run Test Apply**: Runs the `pipeline test-apply` diagnostic tool. This command performs an end-to-end test of the full pipeline — scanning Djinni for a job (or falling back to a dummy job if none is found), running LLM evaluation, generating a CV and cover letter, and producing verbose trace logging to `logs/test-apply.log`. By default it runs in **dry-run mode** (`--dry-run`) so no data is submitted; pass `--dry-run=false` to disable dry-run.
  ```bash
  make run-test-apply
  ```
   The trace log captures raw HTTP requests/responses, LLM prompts and responses, latency measurements, and pipeline step timing, making it invaluable for debugging integration issues. The log file is written to `logs/test-apply.log` relative to the project root.

## Applied-jobs tracker

The bot maintains a persistent registry of jobs that have been applied to, stored in `data/applied_jobs.json`. This registry prevents duplicate evaluations and applications by:

1. **Registry skip check**: Before fetching job details, the bot loads the registry and skips any job whose ID or slug is already recorded.
2. **HTML-based detection**: After fetching job details, the bot checks for the HTML snippet indicating an already-applied status. If detected, the job is skipped and its ID is added to the registry.
3. **Automatic persistence**: After a successful application submission (including retry completions), the job ID is automatically saved to the registry.

The registry is a simple JSON file where keys are job IDs (numeric strings) and values are RFC3339 timestamps of the application moment. The file is written atomically to avoid corruption.
