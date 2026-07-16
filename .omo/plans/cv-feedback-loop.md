# cv-feedback-loop - Work Plan

## TL;DR (For humans)

**What you'll get:** Система, которая автоматически сохраняет каждый ответ рекрутера Djinni в файл, и при накоплении 5 новых ответов — запускает анализ твоего CV через Gemini, выдавая конкретные предложения по улучшению в отдельный файл (который ты читаешь и решаешь сам, что применять).

**Why this approach:** Go-бот только пишет данные (персистит сообщение рекрутера в JSONL при каждом обработанном диалоге). Анализ CV живёт целиком в career-ops рядом с cv.md и reports/ — без лишних зависимостей между компонентами. JSONL — формат append-only, безопасен для конкурентной записи, читаем в git diff.

**What it will NOT do:** Никогда не изменяет cv.md — только читает. Не отправляет запросы сама по себе без накопленного порога. Не требует дополнительных ключей или сервисов — только OPENAI_API_KEY (который уже используется).

**Effort:** Medium  
**Risk:** Low — обе стороны (Go и Node.js) модифицируются минимально, все изменения аддитивны  
**Decisions to sanity-check:** (1) JSONL-формат как shared storage между Go и Node — устраивает? (2) OpenAI-compatible API как модель анализа — через OPENAI_API_KEY + модель OPENAI_MODEL || FREELLMAPI_MODEL || 'gpt-4o-mini'?

Your next move: approve, and then run `/start-work`. Full execution detail follows below.

---

> TL;DR (machine): Medium effort, Low risk. Deliverables: inbox.go persistence patch (Go), analyze-cv-feedback.mjs + modes/cv-feedback.md (Node.js), CLAUDE.md update. Trigger threshold: 5 feedbacks since last analysis.

## Scope
### Must have
- Go `inbox.go`: append recruiter feedback (dialogID, sender, message, timestamp) to `career-ops/data/recruiter-feedback.jsonl` after each processed dialogue (regardless of shouldReply value — всегда, даже если агент решил не отвечать)
- Node `analyze-cv-feedback.mjs`: read `cv.md` + last N feedbacks, call Gemini API, write `reports/cv-improvement-YYYY-MM-DD.md`
- Counter logic: 5 feedbacks with `type=feedback` since last `type=analysis` sentinel triggers auto-analysis
- `modes/cv-feedback.md`: agent mode для ручного запуска анализа
- `CLAUDE.md`: регистрация нового режима cv-feedback

### Must NOT have (guardrails, anti-slop, scope boundaries)
    - NO modifications to `cv.md`, `data/applications.md`, or any report file
    - NO auto-modification of CV — только чтение + запись предложений в отдельный файл
    - NO separate env files or new API keys — только существующий OPENAI_API_KEY
    - NO sending messages or replying to recruiters from the analysis path
    - NO changes to inbox.log format — только добавление recruiter-feedback.jsonl

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: tests-after (manual functional QA — no test framework in project)
- Evidence: `.omo/evidence/cv-feedback-loop-*.md` (dry-run output + JSONL sample)

## Execution strategy
### Parallel execution waves

**Wave 1** (parallel: T1 + T2) — Data layer: persistence + script skeleton  
**Wave 2** (sequential: T3, then T4) — Analysis logic + mode file  
**Wave 3** (sequential: T5) — CLAUDE.md registration

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
| --- | --- | --- | --- |
| 1. (Go: persist feedback) | — | 3. (script reads jsonl) | 2. |
| 2. (Node: script skeleton) | — | 3. (needs the structure) | 1. |
| 3. (Node: full analysis logic) | 1., 2. | 4. | — |
| 4. (mode cv-feedback.md) | 3. (references script) | 5. | — |
| 5. (CLAUDE.md update) | 4. | — | — |

## Todos
> Implementation + Test = ONE todo. Never separate.
<!-- APPEND TASK BATCHES BELOW THIS LINE WITH edit/apply_patch - never rewrite the headers above. -->

- [ ] 1. djinni-bot-go/internal/pipeline/inbox.go: append recruiter message to recruiter-feedback.jsonl after each dialogue processing
  What to do: Inside the `for _, d := range dialogues` loop in `ProcessInbox()`, after the existing per-dialogue processing (after the inner `for {}` loop ends — line ~161), append a JSONL record to `career-ops/data/recruiter-feedback.jsonl`.  
  JSONL record format (one JSON per line, LF-terminated):  
  `{"type":"feedback","ts":"2006-01-02T15:04:05Z07:00","dialog_id":"<d.ID>","sender":"<d.Sender>","message":"<d.Message>"}`  
  Must NOT do: Do NOT write to inbox.log — this is a new separate file. Do NOT persist LLM-generated reply here.  
  Implementation detail:  
  - feedbackFile path = `filepath.Join(contextDir, "data", "recruiter-feedback.jsonl")`  
  - Create record struct `FeedbackRecord{Type, TS, DialogID, Sender, Message string}` (local, no new file needed)  
  - Marshal to JSON, open file with `os.O_APPEND|os.O_CREATE|os.O_WRONLY|0644`, write line + `\n`  
  - Skip write if dryRun == true (consistent with inbox.log behavior at line 152)  
  - If write fails, append warning to `logs` — do NOT abort the whole inbox run  
  Parallelization: Wave 1 | Blocked by: nothing | Blocks: T3  
  References: `djinni-bot-go/internal/pipeline/inbox.go:27,43-165` (full function), `djinni-bot-go/internal/pipeline/inbox.go:152-159` (inbox.log pattern to mirror)  
  Acceptance criteria: After a dry-run test (or real run), `career-ops/data/recruiter-feedback.jsonl` contains one valid JSON line per processed dialogue with correct fields. `cat career-ops/data/recruiter-feedback.jsonl | python3 -c "import sys,json;[json.loads(l) for l in sys.stdin]"` exits 0.  
  QA scenarios:  
  - happy: run `go run ./... inbox --dry-run` (or equivalent), confirm jsonl file created/appended with correct structure  
  - failure: if GEMINI fails mid-dialogue, the feedback for dialogues processed before the failure must still be written  
  Evidence: `.omo/evidence/cv-feedback-loop-T1-jsonl-sample.txt`  
  Commit: Y | feat(inbox): persist recruiter feedback to recruiter-feedback.jsonl

- [ ] 2. career-ops/analyze-cv-feedback.mjs: create script skeleton with CLI interface and file I/O
  What to do: Create `career-ops/analyze-cv-feedback.mjs` with:  
  1. Shebang `#!/usr/bin/env node`  
  2. `CAREER_OPS = dirname(fileURLToPath(import.meta.url))` (same pattern as analyze-patterns.mjs:18)  
  3. Constants: `FEEDBACK_FILE`, `CV_FILE = join(CAREER_OPS, 'cv.md')`, `REPORTS_DIR`, `THRESHOLD = 5`  
  4. Function `parseFeedback()` — reads recruiter-feedback.jsonl line by line, returns `{feedbacks: [...], lastAnalysisIndex: N}` where feedbacks = lines with `type=feedback` after last `type=analysis` line  
  5. Function `countPendingFeedbacks()` — returns count of feedbacks since last analysis sentinel  
  6. CLI args: `--force` (skip threshold check), `--dry-run` (print prompt without calling API)  
  7. Main guard: `if (countPendingFeedbacks() < THRESHOLD && !forceFlag) { console.log('Not enough feedbacks yet: N/5. Run with --force to override.'); process.exit(0); }`  
  Must NOT do: Do NOT call any LLM in this todo — just skeleton. Do NOT create a separate package.json.  
  Parallelization: Wave 1 | Blocked by: nothing | Blocks: T3  
  References: `career-ops/analyze-patterns.mjs:1-31` (pattern for CAREER_OPS, file resolution), `career-ops/modes/patterns.md:16-20` (threshold check UX)  
  Acceptance criteria: `node career-ops/analyze-cv-feedback.mjs` exits 0 with "Not enough feedbacks yet" message (when no jsonl exists). `node career-ops/analyze-cv-feedback.mjs --force --dry-run` prints "dry-run: would call LLM with prompt [preview]".  
  QA scenarios:  
  - happy: run both variants above, confirm expected output  
  - failure: if cv.md missing, script exits with clear error message  
  Evidence: `.omo/evidence/cv-feedback-loop-T2-dry-run.txt`  
  Commit: Y | feat(career-ops): add analyze-cv-feedback.mjs skeleton

- [ ] 3. career-ops/analyze-cv-feedback.mjs: implement full OpenAI-compatible analysis + report writing
  What to do: Extend the script from T2 with:  
  1. Read `cv.md` via `readFileSync`  
  2. Build analysis prompt:  
     - System: "You are a career coach. Analyze the following CV and recruiter feedback messages. Identify patterns in recruiter reactions, suggest specific improvements to the CV. Be concrete: quote exact CV sections that should change, provide improved alternatives. Output in Markdown with sections: ## Executive Summary, ## Patterns Observed, ## Specific Recommendations (numbered, each with: current text, issue, suggested replacement)."  
     - User: CV content + last N feedback messages (include sender, message, date)  
  3. Call OpenAI-compatible API via native `fetch` (NO SDK needed — Node 18+ has built-in fetch):  
     ```js
     const baseUrl = process.env.FREELLMAPI_BASE_URL?.replace(/\/$/, '') ?? 'https://api.openai.com/v1';
     const apiKey = process.env.OPENAI_API_KEY ?? process.env.LLM_API_KEY;
     const model = process.env.OPENAI_MODEL ?? process.env.FREELLMAPI_MODEL ?? 'gpt-4o-mini';
     const resp = await fetch(`${baseUrl}/chat/completions`, {
       method: 'POST',
       headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${apiKey}` },
       body: JSON.stringify({ model, messages: [{ role: 'system', content: system }, { role: 'user', content: user }] })
     });
     const data = await resp.json();
     const text = data.choices?.[0]?.message?.content;
     ```
     - If `OPENAI_API_KEY` missing AND `LLM_API_KEY` missing → exit with clear error message  
     - If resp.ok is false → exit with HTTP status + data.error.message  
  4. Write output to `reports/cv-improvement-YYYY-MM-DD.md` with frontmatter:  
     ```  
     ---  
     date: YYYY-MM-DD  
     feedbacks_analyzed: N  
     model: <model name>  
     ---  
     ```  
  5. Append sentinel `{"type":"analysis","ts":"...","feedbacks_count":N,"report":"reports/cv-improvement-YYYY-MM-DD.md"}` to `recruiter-feedback.jsonl`  
  6. Print "CV improvement report written: reports/cv-improvement-YYYY-MM-DD.md"  
  Must NOT do: Do NOT write to cv.md. Do NOT delete or modify existing feedbacks in jsonl. Do NOT install or import any npm SDK.  
  Parallelization: Wave 2 | Blocked by: T1 (jsonl format confirmed), T2 (skeleton) | Blocks: T4  
  References: `career-ops/analyze-patterns.mjs:1-31` (file I/O pattern), `djinni-bot-go/internal/llm/factory.go:96-118` (OpenAI engine pattern to port to JS fetch), `career-ops/modes/patterns.md:49-53` (report output pattern)  
  Acceptance criteria: `OPENAI_API_KEY=sk-... node career-ops/analyze-cv-feedback.mjs --force` with real key writes `reports/cv-improvement-YYYY-MM-DD.md` with valid markdown and frontmatter. File contains all 3 required sections. Sentinel appended to jsonl. `node --check career-ops/analyze-cv-feedback.mjs` exits 0.  
  QA scenarios:  
  - happy: `--force` run with real OPENAI_API_KEY, file exists and has content  
  - failure: missing OPENAI_API_KEY AND LLM_API_KEY → clear error message, no partial file written  
  Evidence: `.omo/evidence/cv-feedback-loop-T3-report-sample.md`  
  Commit: Y | feat(career-ops): implement CV analysis with OpenAI-compatible API in analyze-cv-feedback.mjs

- [ ] 4. career-ops/modes/cv-feedback.md: create agent mode for manual CV feedback analysis
  What to do: Create `career-ops/modes/cv-feedback.md` following the exact structure of `modes/patterns.md`. Include:  
  - `# Mode: cv-feedback -- CV Improvement Analyzer`  
  - Purpose section: analyze recruiter feedback to suggest CV improvements  
  - Inputs: `data/recruiter-feedback.jsonl`, `cv.md`, `config/profile.yml`  
  - Minimum Threshold: same 5-feedback check, user-friendly message if not met  
  - Step 1: Execute `node analyze-cv-feedback.mjs [--force if user requested]`  
  - Step 2: Present summary of top recommendations (3-5 bullet points)  
  - Step 3: Show full report path  
  - Step 4: Remind user CV changes are manual — "I'll present the suggestions; you decide what to apply."  
  - Note: `--force` flag usage explained (bypass threshold)  
  Must NOT do: Do NOT instruct agent to modify cv.md.  
  Parallelization: Wave 2 | Blocked by: T3 | Blocks: T5  
  References: `career-ops/modes/patterns.md` (exact template to mirror), `career-ops/modes/inbox-reply.md` (tone reference)  
  Acceptance criteria: File exists at `career-ops/modes/cv-feedback.md`. Contains all 4 steps. Does NOT mention editing cv.md. `grep -i "edit cv\|modify cv\|write cv" career-ops/modes/cv-feedback.md` returns empty.  
  QA scenarios:  
  - happy: file created with correct structure, passes grep check above  
  - failure: n/a (static file write)  
  Evidence: `.omo/evidence/cv-feedback-loop-T4-mode-grep.txt`  
  Commit: Y | feat(career-ops): add cv-feedback mode for CV improvement analysis

- [ ] 5. career-ops/CLAUDE.md: register cv-feedback mode in agent routing
  What to do: Read `career-ops/CLAUDE.md` to find where modes are listed/registered. Add `cv-feedback` to the mode registry. The entry should describe: trigger phrase "analyze cv feedback" or "cv improvement", description "analyzes recruiter responses to suggest CV improvements", and reference to `modes/cv-feedback.md`. Follow existing format exactly.  
  Must NOT do: Do NOT change any other mode behavior. Do NOT alter data contract section.  
  Parallelization: Wave 3 | Blocked by: T4 | Blocks: nothing  
  References: `career-ops/CLAUDE.md` (read first to find modes section format), `career-ops/modes/cv-feedback.md` (T4 output)  
  Acceptance criteria: `grep -n "cv-feedback" career-ops/CLAUDE.md` returns at least 1 match. Mode listed in the same section as other modes (patterns, followup, inbox-reply).  
  QA scenarios:  
  - happy: grep confirms registration, no other mode entries changed  
  - failure: if CLAUDE.md format is unexpected, add entry in the most similar existing section  
  Evidence: `.omo/evidence/cv-feedback-loop-T5-claude-grep.txt`  
  Commit: Y | docs(career-ops): register cv-feedback mode in CLAUDE.md

## Final verification wave
> Runs in parallel after ALL todos. ALL must APPROVE. Surface results and wait for the user's explicit okay before declaring complete.
- [ ] F1. Plan compliance audit — verify T1-T5 deliverables exist, no cv.md modification, sentinel written correctly
- [ ] F2. Code quality review — inbox.go change compiles (`go build ./...`), analyze-cv-feedback.mjs passes `node --check`
- [ ] F3. Real manual QA — end-to-end dry-run: inject 5 fake feedback lines into jsonl, run `node analyze-cv-feedback.mjs`, confirm report generated with correct structure
- [ ] F4. Scope fidelity — confirm cv.md unmodified, applications.md unmodified, inbox.log unmodified

## Commit strategy
5 commits, one per todo. All in feature branch or directly on main (project has no branch conventions observed).

## Success criteria
- `career-ops/data/recruiter-feedback.jsonl` accumulates feedback after each inbox run
- `node career-ops/analyze-cv-feedback.mjs` respects threshold and exits cleanly when <5 feedbacks
- `node career-ops/analyze-cv-feedback.mjs --force` generates a valid report in `reports/`
- Sentinel line appended to jsonl after each analysis
- Agent responds to "analyze cv feedback" by running the mode
- cv.md, applications.md, inbox.log — all unmodified by this feature
