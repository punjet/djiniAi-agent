# Djinni Bot Go

This directory contains the Go implementation of the Djinni bot, including the `career-ops` CLI tool for evaluating job descriptions using an LLM.

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
