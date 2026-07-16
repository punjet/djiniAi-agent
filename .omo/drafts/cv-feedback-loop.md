---
slug: cv-feedback-loop
status: awaiting-approval
intent: clear
review_required: false
pending-action: write .omo/plans/cv-feedback-loop.md
approach: >
  Two-component feature: (1) Go side — persist recruiter messages from inbox.go
  into career-ops/data/recruiter-feedback.jsonl on every processed dialogue.
  (2) Node.js side — new analyze-cv-feedback.mjs script + modes/cv-feedback.md mode
  that reads cv.md + last N feedbacks, calls Gemini API, writes reports/cv-improvement-{date}.md.
  Trigger: 5 feedbacks accumulated since last analysis (tracked by sentinel field in jsonl).
---

# Draft: cv-feedback-loop

## Components (topology ledger)

| id | outcome | status | evidence path |
|----|---------|--------|---------------|
| A | Go: persist d.Message + metadata to recruiter-feedback.jsonl | active | djinni-bot-go/internal/pipeline/inbox.go:150-160 |
| B | Node: analyze-cv-feedback.mjs script (reads feedback + cv.md, calls Gemini, writes report) | active | career-ops/analyze-patterns.mjs (reference) |
| C | Node: modes/cv-feedback.md agent mode (manual trigger) | active | career-ops/modes/patterns.md (reference) |
| D | career-ops/CLAUDE.md: register cv-feedback mode | active | career-ops/CLAUDE.md |

## Open assumptions (announced defaults)

| assumption | adopted default | rationale | reversible? |
|-----------|-----------------|-----------|-------------|
| Feedback storage format | JSONL (recruiter-feedback.jsonl) | Append-only, machine-readable, no schema migration | Yes |
| Counter mechanism | Count uniq lines in jsonl since last `last_analyzed_at` sentinel | Simpler than reading applications.md statuses | Yes |
| Last-analysis sentinel | Last line with type=analysis in recruiter-feedback.jsonl | No separate state file | Yes |
| LLM API for Node.js analysis | OpenAI-compatible via native fetch — OPENAI_API_KEY or LLM_API_KEY; baseUrl from FREELLMAPI_BASE_URL or https://api.openai.com/v1; model from OPENAI_MODEL \|\| FREELLMAPI_MODEL \|\| 'gpt-4o-mini' | User is on OpenAI token; no SDK install needed; same API shape that Go factory.go already uses | Yes — env vars only |
| Analysis output path | career-ops/reports/cv-improvement-YYYY-MM-DD.md | Consistent with patterns.md output | Yes |
| Trigger threshold | 5 feedbacks (new since last analysis) | Matches patterns.md MIN_THRESHOLD convention | Yes |
| Node.js LLM call | Native fetch (no npm SDK) to OpenAI-compatible endpoint | career-ops has no SDK deps; fetch is built-in in Node 18+ | Yes — could use curl |

## Findings (cited - path:lines)

- inbox.go:43 — logFile = filepath.Join(contextDir, "data", "inbox.log")
- inbox.go:150-160 — logs d.Sender + finalReply, but d.Message is NEVER written anywhere
- inbox.go:45 — `for _, d := range dialogues` — d.Message available here
- inbox.go:27 — contextDir passed in as parameter (resolves to career-ops/)
- gemini.go:22-26 — GeminiConfig{APIKey, Model, Temperature, MaxOutputTokens}
- analyze-patterns.mjs:1-80 — CAREER_OPS = dirname(import.meta.url), reads from same dir
- modes/patterns.md:17 — MIN_THRESHOLD = 5 entries beyond Evaluated
- api/models.go — Dialogue{ID, Sender, Message} — contains one recruiter message

## Decisions (with rationale)

1. **JSONL over SQLite** — recruiter-feedback.jsonl lives in career-ops/data/ (same as inbox.log).
   Go appends to it; Node reads it. No shared DB needed. JSONL is append-safe and Git-trackable.

2. **Trigger in analyze-cv-feedback.mjs, not in inbox.go** — Go side just persists. Analysis logic 
   lives in Node.js career-ops where cv.md and reports/ are. Clean separation.

3. **OpenAI-compatible API via native fetch in Node.js** — реиспользует OPENAI_API_KEY или LLM_API_KEY из .env.
   baseUrl = FREELLMAPI_BASE_URL (если задан) или https://api.openai.com/v1.
   Модель = OPENAI_MODEL || FREELLMAPI_MODEL || 'gpt-4o-mini'. Никаких npm SDK — только встроенный fetch.

4. **sentinel via type field** — each JSONL line has {type: "feedback"|"analysis", ...}.
   Counter = lines with type=feedback since last type=analysis line.

5. **Mode cv-feedback.md is manual trigger only** — auto-trigger is inside analyze-cv-feedback.mjs.
   Agent mode lets user manually invoke "analyze my CV feedback" without threshold.

## Scope IN

- djinni-bot-go/internal/pipeline/inbox.go — add feedback persistence block
- career-ops/data/recruiter-feedback.jsonl — new file (auto-created)
- career-ops/analyze-cv-feedback.mjs — new script
- career-ops/modes/cv-feedback.md — new mode
- career-ops/CLAUDE.md — register new mode

## Scope OUT (Must NOT have)

- NEVER modify cv.md (user-layer — read-only for agents)
- NEVER modify data/applications.md or reports/* (user-layer)  
- NEVER send LLM requests from Go side for CV analysis (Go side = persistence only)
- NEVER install npm SDK for LLM — использовать только native fetch
- NEVER create a separate .env or config file — reuse OPENAI_API_KEY / LLM_API_KEY
- NEVER require manual cron or external scheduler — self-contained check inside .mjs

## Open questions

None — all resolved by exploration.

## Approval gate
status: awaiting-approval
pending-action: write full todos into .omo/plans/cv-feedback-loop.md
