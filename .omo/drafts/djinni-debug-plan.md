# Draft: djinni-bot-go Debug & Test Plan

## Meta
- intent: unclear (project is broken, no clear entry point known)
- review_required: false
- status: awaiting-approval
- slug: djinni-debug-plan
- created: 2026-07-02

## Problem Statement
The djinni-bot-go Go CLI bot is not working. The project is complex — a polyglot system (Go + Node.js) with many layers of integration. The user needs a diagnosis of why it fails, a map of all weak spots, and a plan for tests + debugging.

## Findings

### Critical Blockers (will prevent `go build` or runtime)

1. **go.mod declares go 1.26.2** — this version does not exist (as of July 2026 Go is at 1.22/1.23). This WILL cause `go build` to fail with "version is too new" on older toolchains.
   - File: `djinni-bot-go/go.mod`, line 3

2. **Missing .env file** — `LoadConfig()` calls `godotenv.Load()` but there is no `.env` in `djinni-bot-go/`. Without DJINNI_SESSIONID + DJINNI_CSRFTOKEN, `pipeline run` fails immediately. Without GEMINI_API_KEY, the gemini engine fails.

3. **Node.js cross-language dependency** — `GenerateCoverLetter` and `GenerateCustomCV` both call `exec.Command("node", ...)` for PDF generation. If Node.js is not installed or `career-ops/` scripts are missing, the entire apply flow fails.

4. **career-ops/ context directory must exist** with exact files:
   - `modes/_shared.md`, `modes/oferta.md`, `modes/_profile.md`
   - `cv.md`
   - `config/profile.yml`
   - `portals.yml` (optional, fallback exists)
   - `data/scan-history.tsv` (auto-created)
   - `data/applications.md` (for dedup)

### Structural Weaknesses (crutches / fragile code)

5. **HTML parsed via `regexp`** — all Djinni page parsing uses regex on HTML. Any UI change on Djinni.co breaks it silently. Located in:
   - `internal/extractor/regex.go` (ExtractJobs, ExtractDashboardJobs, ExtractJobDetails)
   - `internal/api/inbox.go` (GetUnreadMessages, ReplyToMessage)

6. **Apply button detection via string presence** — `GetJobDetails` checks `strings.Contains(html, "js-inbox-toggle-reply-form")`. If Djinni renames this CSS class, all jobs appear "blocked".

7. **`applied=ok` redirect detection** — `ApplyToJob` checks if final URL contains `"applied=ok"`. Djinni could change this redirect pattern at any time.

8. **os.Stdout swap in evaluate.go** — lines 137-143 do `os.Stdout = os.Stderr` then restore. This is goroutine-unsafe and will cause test failures or data races in concurrent scenarios.

9. **Hardcoded search queries** — 13 search queries in `pipeline/scanner.go:118-132` are NOT configurable from portals.yml. Must change Go code to add/remove queries.

10. **No HTTP retry** — single attempt for every Djinni API call. Rate limits or transient errors cause immediate failure.

11. **Daemon mode has no daily cap** — the `--limit` flag is only checked in one-shot mode. In `--daemon` mode, there is no limit on applications per day (Djinni likely has one ~15/day).

12. **freellmapi hardcoded delays** — 30s/60s/20s sleep calls in `processJobItem` are hardcoded.

13. **Session cookie expiry** — no mechanism to detect/refresh expired Djinni cookies. When they expire, all HTTP calls silently return 200 with a login redirect page (which regex parsing then fails on silently).

### LLM Integration Risks

14. **LLM output validation is brittle** — `validateEvaluationShape` checks for blocks A-G using regex on markdown headers. If the LLM uses different formatting (e.g., "**A.**" vs "## A."), validation fails and the job is skipped.

15. **JSON parsing in covergen** — `GenerateCoverLetter` manually strips `{...}` from LLM response. If LLM outputs malformed JSON or extra text, `json.Unmarshal` fails and the application is not submitted.

16. **CV HTML generation is a black box** — `GenerateCustomCV` calls `scripts/generate-cv-html.mjs` and expects a specific filename pattern (`test-cv-{company}.html`). If the company name has special chars, this path could break.

## Decisions (Adopted Defaults, No Interview Required)

- Test framework: standard `go test` (already in use in existing test files)
- Test scope: unit tests first, integration stubs second (no real Djinni network calls)
- Mock strategy: interface-based mocks for `llm.Provider` (already an interface), HTML fixtures for extractor tests
- Plan focus: diagnose + fix blockers first, then add missing tests, then document weak spots

## Approval Status
Status: plan-complete — HAR analysis done, Phase 5 (HTML→goquery migration) added.

## HAR Analysis Findings (Critical)

- Djinni.co has NO JSON REST API for job listing, search, apply, or inbox
- All `application/json` responses in HAR files are Intercom/Sentry analytics
- HTMX endpoint `/jobs/{id}/similar-jobs/` returns HTML fragment, not JSON
- The original "use network requests" intent must be re-interpreted as:
  1. Expand `application/ld+json` parsing (already done, extend it)
  2. Replace regexp with goquery CSS-selector parsing (much more reliable)
  3. Improve apply success detection via both redirect URL AND response body
- Phase 5 tasks (501-507) added to plan accordingly
