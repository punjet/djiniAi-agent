# tg-dashboard - Work Plan

## TL;DR (For humans)

**What you'll get:** Telegram-бот превращается из системы «уведомлений» в полноценный пульт управления: теперь он постоянно слушает команды и отвечает на них в реальном времени. Добавляется «тревожная кнопка» `/panic` — одна команда немедленно останавливает всё что делает бот. В режиме демона бот показывает живой статус-экран (редактируется каждые 30 секунд). **Каждая заявка на работу теперь требует твоего подтверждения в Telegram** — бот показывает ссылку на вакансию, cover letter и имя CV, ты можешь нажать ✅ Подать, ✏️ Редактировать (написать инструкцию → AI перегенерирует → снова подтвердить) или ❌ Отклонить. **Каждый ответ рекрутёру** тоже идёт через Telegram: бот показывает тред разговора + черновик ответа, ты подтверждаешь или правишь.

**Why this approach:** Постоянный горутин-поллер (один на весь процесс) — минимальный рефактор без смены инфраструктуры; atomic bool для panic-stop — потокобезопасно и без гонок; всё остальное остаётся как было.

**What it will NOT do:** Нет webhook / HTTP сервера. Нет внешних баз данных. Нет изменений в Djinni-парсере или JS career-ops. Нет нового бинарника.

**Effort:** Large
**Risk:** Medium — рефактор polling и добавление concurrent goroutine требует аккуратности

**Decisions I made for you:**
- Persistent bot = горутин в том же процессе (не отдельный сервис)
- Panic-stop = `*atomic.Bool` — проверяется в каждом цикле
- Статус-борд = один pinned message, редактируется каждые 30с
- Apply review = **обязательное** (без флага; каждый apply ждёт ответа из Telegram)
- Cover letter = полный текст в Telegram-сообщении (обрезается до 3000 символов если длиннее)
- Edit cover letter flow = кнопка ✏️ → пользователь пишет инструкцию → LLM перегенерирует → новый вариант снова в Telegram → цикл до Подать или Отклонить
- TG_BOT_TOKEN / TG_CHAT_ID документированы в .env.example (env vars не меняются)
- Никакого webhook — polling достаточен для одного пользователя

Your next move: напиши **approve** чтобы запустить выполнение, или скажи что изменить.

---

> TL;DR (machine): Large effort, Medium risk. Delivers persistent TelegramBot goroutine, /panic stop, /status /stop /report commands, live daemon status board, thread-safe offset, mandatory AskUserForApplyReview with cover-letter regeneration loop, confirmed human-in-the-loop for inbox replies.

## Scope
### Must have
1. `TelegramBot` struct in `internal/notify/` — owns goroutine, offset, command dispatch
2. Bot commands: `/start` (hello), `/status` (current phase), `/stop` (graceful shutdown), `/panic` (immediate stop all), `/report` (last run summary)
3. Panic-stop: `*atomic.Bool` checked in ProcessInbox loop, processJobItem, daemon loop
4. Daemon mode: starts TelegramBot, pins live status message (edited every 30s)
5. **Mandatory** apply review: every `api.ApplyToJob` call requires prior Telegram confirmation — bot shows job URL + company + role + score + CV filename + full cover letter (up to 3000 chars); buttons: ✅ Submit / ✏️ Edit / ❌ Reject. On ✏️: user writes instruction → LLM regenerates cover letter → new draft back to Telegram → repeat until ✅ or ❌. No `--review-apply` flag; always required.
6. **Confirmed** inbox reply review: AskUserForInboxReview is already called in ProcessInbox (line 170); this plan ensures it continues to work post-T7 refactor and is explicitly listed as a must-have deliverable.
7. Thread-safe polling: remove `var lastOffset int64 = 0` package global → TelegramBot field
8. `.env.example` documents TG_BOT_TOKEN + TG_CHAT_ID
9. Tests: command parsing unit test, panic-stop propagation test, apply-review loop unit test

### Must NOT have (guardrails, anti-scope)
- No `--review-apply` flag (apply review is always on, flag removed from plan)
- No Webhook / HTTP listener
- No external database or Redis
- No changes to Djinni scraping / HTML parsers
- No changes to scanner, evaluator, dedup, or career-ops JS
- No new CLI binary
- No goroutine-per-dialogue
- No change to inbox.log format
- No change to AskUserForInboxReview return type

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: tests-after (existing project uses tests-after pattern, no TDD convention found)
- Framework: standard `go test` with `-race` flag for concurrency bugs
- Evidence: .omo/evidence/task-TN-tg-dashboard.txt per task

## Execution strategy
### Parallel execution waves

**Wave 1** (independent foundations):
- T1: TelegramBot struct + goroutine infrastructure
- T8: .env.example + TG config documentation

**Wave 2** (depends on T1):
- T2: Command router (/start /status /stop /panic /report)
- T7: Thread-safe lastOffset migration (AskUserForInboxReview + AskUserForApplyReview get bot param)

**Wave 3** (depends on T1 + T2):
- T3: Panic-stop flag propagation into all loops (also wires bot into processJobItem)

**Wave 4** (depends on T1 + T2 + T3 + T7, parallel):
- T4: Live status board in daemon mode
- T5: Mandatory apply review loop (cover letter show + edit + regenerate) — depends on T3+T7 (bot param in processJobItem + lastOffset migrated)
- T6: /report command wiring (SetLastSummary call sites)

**Wave 5** (depends on T7):
- T9: Verify inbox reply human-in-the-loop end-to-end after refactor

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
| --- | --- | --- | --- |
| T1 | — | T2, T3, T4, T5, T6, T7 | T8 |
| T2 | T1 | T3, T4, T6 | T7 |
| T3 | T1, T2 | T4, T5 | — |
| T4 | T1, T2, T3 | — | T5, T6 |
| T5 | T1, T2, T3, T7 | — | T4, T6 |
| T6 | T1, T2 | — | T4, T5 |
| T7 | T1 | T5, T9 | T2 |
| T8 | — | — | T1 |
| T9 | T7 | — | T4, T5, T6 |

## Todos
> Implementation + Test = ONE todo. Never separate.
<!-- APPEND TASK BATCHES BELOW THIS LINE WITH edit/apply_patch - never rewrite the headers above. -->

- [x] 1. Create `TelegramBot` struct in `internal/notify/bot.go` with persistent goroutine polling
  What to do: Create new file `internal/notify/bot.go` in package `notify`. Define:
  ```go
  type TelegramBot struct {
      lastOffset  int64          // replaces package global in interactive.go
      panicStop   *atomic.Bool   // shared with pipeline
      lastSummary string         // stores last run summary for /report
      statusMsgID int64          // message ID of pinned status message
      mu          sync.Mutex     // protects lastSummary
      done        chan struct{}   // closed on Stop()
      commands    chan BotCommand // buffered channel, capacity 16
  }
  type BotCommand struct { Name string; CallbackQueryID string }
  func NewTelegramBot() *TelegramBot
  func (b *TelegramBot) Start(ctx context.Context) // launches goroutine, drains stale updates
  func (b *TelegramBot) Stop()                     // closes done channel, signals goroutine to exit
  func (b *TelegramBot) PanicStop() *atomic.Bool   // returns the shared panicStop flag
  func (b *TelegramBot) SetLastSummary(s string)   // thread-safe setter
  func (b *TelegramBot) Commands() <-chan BotCommand
  func (b *TelegramBot) GetUpdates() ([]TGUpdate, error) // delegates to notify.GetUpdates(b.lastOffset)
  ```
  The goroutine in Start() must:
  - Drain stale updates at startup (same as InitTelegramOffset — call GetUpdates(0) once, advance b.lastOffset)
  - Loop: select on ctx.Done() and done channel for exit; call GetUpdates(b.lastOffset) every 2s
  - Dispatch: if update is a Message with text starting "/" → parse command → send to b.commands channel (non-blocking, drop if full)
  - Advance b.lastOffset on every update
  Must NOT: start more than one goroutine per Start() call; must NOT call GetUpdates if TG_BOT_TOKEN is empty (return early)
  References: `internal/notify/telegram.go:1-290` (GetUpdates, TGUpdate types); `internal/pipeline/interactive.go:1` (lastOffset global to replace)
  Acceptance criteria: `go build ./internal/notify/...` exits 0; `go test ./internal/notify/... -race -count=1` exits 0; file internal/notify/bot.go exists with TelegramBot type
  QA scenarios:
    happy: `go test ./internal/notify/... -v -run TestTelegramBot -race` passes; if TG_BOT_TOKEN empty, Start() returns without launching goroutine
    failure: `go test -race` must find no data race on lastOffset
    Evidence: .omo/evidence/task-T1-tg-dashboard.txt
  Commit: Y | feat(tg): add TelegramBot struct with persistent polling goroutine

- [x] 2. Implement command router inside TelegramBot goroutine
  What to do: Inside the goroutine loop in `internal/notify/bot.go`, after receiving updates, route text messages that start with "/" to a `dispatchCommand(text string)` method:
  ```
  /start  → SendTelegramMessage("🤖 Career-ops bot is running. Commands: /status /stop /panic /report")
  /status → sends b.buildStatusText() (current phase + last summary preview)
  /stop   → SendTelegramMessage("⏹ Graceful shutdown requested…"); close(b.done)
  /panic  → b.panicStop.Store(true); SendTelegramMessage("🚨 PANIC STOP activated. All operations halted.")
  /report → SendTelegramMessage(b.lastSummary or "No report yet.")
  unknown → SendTelegramMessage("Unknown command. Try /status /stop /panic /report")
  ```
  `buildStatusText()` returns a string with current time, panicStop state, lastSummary first 200 chars.
  MUST NOT route callback_query events here — those are still handled inline in interactive.go.
  References: `internal/notify/bot.go` (T1 output); `internal/notify/telegram.go:SendTelegramMessage`; `cmd/career-ops/pipeline.go:176` (lastSummary setter)
  Acceptance criteria: `go build ./internal/notify/...` exits 0; unit test `TestBotCommandRouting` passes — simulate fake Update with Message.Text="/panic", assert panicStop.Load()==true after dispatch
  QA scenarios:
    happy: TestBotCommandRouting passes with -race flag
    failure: /unknown command returns "Unknown command" string (assert in test)
    Evidence: .omo/evidence/task-T2-tg-dashboard.txt
  Commit: Y | feat(tg): add /start /status /stop /panic /report command router

- [x] 3. Propagate panic-stop flag into ProcessInbox, processJobItem, and daemon loop
  What to do:
  1. `internal/pipeline/inbox.go` `ProcessInbox` signature: add `panicStop *atomic.Bool` parameter AFTER existing `dryRun bool`. At the top of the dialogue loop (line 45), add:
     ```go
     if panicStop != nil && panicStop.Load() {
         log.Println("[inbox] panic stop activated, halting")
         break
     }
     ```
  2. `cmd/career-ops/pipeline.go` `processJobItem`: add `panicStop *atomic.Bool` parameter; check it at the top of the function before any work.
  3. `cmd/career-ops/pipeline.go` daemon loop (`runDaemonMode`): add `panicStop *atomic.Bool` parameter; check it in the select's default case each iteration.
  4. All callers of ProcessInbox and processJobItem must pass the panicStop from the TelegramBot (or nil in tests).
  5. The ProcessInbox caller (`runPipelineInbox`, pipeline.go:445) must: create TelegramBot, call b.Start(ctx), defer b.Stop(), pass b.PanicStop() to ProcessInbox.
  Must NOT change inbox.log format. Must NOT change AskUserForInboxReview signature.
  References: `internal/pipeline/inbox.go:45` (dialogue loop); `cmd/career-ops/pipeline.go:309` (processJobItem); `cmd/career-ops/pipeline.go:362` (runDaemonMode); `cmd/career-ops/pipeline.go:470` (ProcessInbox caller)
  Acceptance criteria: `go build ./...` exits 0; `go vet ./...` exits 0; `go test ./internal/pipeline/... -race -count=1` exits 0; grep confirms `panicStop` appears in inbox.go loop
  QA scenarios:
    happy: go test passes, including existing TestLoadSeenIDs and TestGetThreadMessages
    failure: set panicStop=true before loop → assert loop exits immediately (new TestPanicStop unit test)
    Evidence: .omo/evidence/task-T3-tg-dashboard.txt
  Commit: Y | feat(tg): propagate panic-stop flag into all pipeline loops

- [x] 4. Live status message in daemon mode (edited every 30s)
  What to do: In `cmd/career-ops/pipeline.go` `runDaemonMode`, after TelegramBot.Start() (wired in T3), pin a status message:
  1. At daemon start: `statusMsgID, _ := notify.SendInlineKeyboard("🤖 Career-ops daemon started...", nil)` then store msgID in bot.statusMsgID.
  2. Launch a second goroutine inside runDaemonMode:
     ```go
     go func() {
         ticker := time.NewTicker(30 * time.Second)
         defer ticker.Stop()
         for {
             select {
             case <-ctx.Done(): return
             case <-ticker.C:
                 _ = notify.EditMessageText(bot.statusMsgID, bot.buildStatusText())
             }
         }
     }()
     ```
  3. On daemon shutdown (sigChan or panicStop), edit status message to "⏹ Daemon stopped." + final summary.
  Must NOT: start this goroutine if TG_BOT_TOKEN is empty (guard with `if os.Getenv("TG_BOT_TOKEN") != ""`).
  References: `cmd/career-ops/pipeline.go:362-442` (runDaemonMode); `internal/notify/telegram.go:EditMessageText`; `internal/notify/bot.go:buildStatusText` (T2 output)
  Acceptance criteria: `go build ./...` exits 0; `go vet ./...` exits 0; grep confirms `NewTicker(30` in pipeline.go
  QA scenarios:
    happy: `go build ./... && go vet ./...` passes; if TG_BOT_TOKEN empty, goroutine is not started (guard present in source)
    failure: goroutine does not leak on ctx.Done — verified by race detector: `go test -race ./...` exits 0
    Evidence: .omo/evidence/task-T4-tg-dashboard.txt
  Commit: Y | feat(tg): add live status board edited every 30s in daemon mode

- [x] 5. Mandatory apply review via Telegram with cover letter regeneration loop and add deep logging for debugging cover letter and token issues
   What to do:
   **A. Extend `AskUserForApplyReview` signature** in `internal/pipeline/interactive.go`:
   ```go
   func AskUserForApplyReview(
       ctx context.Context,
       bot *notify.TelegramBot,      // replaces package-global offset (wired after T7)
       company, role, jobURL string,
       score float64,
       cvFileName string,            // e.g. "CV-Kyrylo-Kirov-Company.pdf"
       coverLetter string,           // full text, shown up to 3000 chars in TG message
       jobSlug string,
   ) (approvedCoverLetter string, approved bool, err error)
   ```
   Return: `approvedCoverLetter` is the final cover letter text after any edits (may differ from input).

   **B. Telegram card format:**
   ```
   📋 *Job Application Review*
   🏢 Company: {company}
   💼 Role: {role}
   🔗 {jobURL}
   ⭐ Score: {score}
   📄 CV: {cvFileName}

   *Cover Letter:*
   {coverLetter[:3000]}{"..." if truncated}
   ```
   Buttons row 1: [✅ Submit] [✏️ Edit] [❌ Reject]

   **C. Edit loop** (mirrors inbox "explain to AI" pattern):
   - On ✏️ Edit: bot sends message "✏️ Напиши инструкцию для редактирования cover letter:" → call `waitForUserMessage(ctx)` → get user instruction text
   - Pass instruction back to caller via return value (e.g. return `"edit:<instruction>"`, false, nil)
   - Caller in processJobItem re-generates cover letter via LLM using the instruction, then calls `AskUserForApplyReview` again with new text → repeat until ✅ or ❌
   - On ✅ Submit: return (coverLetter, true, nil)
   - On ❌ Reject: return ("", false, nil) → processJobItem skips apply, logs skip

   **D. Wire into `processJobItem`** in `cmd/career-ops/pipeline.go`:
   - Remove `--review-apply` flag (no flag check)
   - After cover letter generation (line ~270 where `introMsg` is built), enter review loop:
     ```go
     currentCoverLetter := introMsg
     for {
         result, approved, err := pipeline.AskUserForApplyReview(ctx, bot, ...)
         if err != nil { return false, err }
         if !approved { return false, nil }     // ❌ Rejected
         if strings.HasPrefix(result, "edit:") {
             instruction := strings.TrimPrefix(result, "edit:")
             currentCoverLetter, err = regenerateCoverLetter(ctx, cfg, details, res, instruction)
             if err != nil { return false, err }
             continue
         }
         introMsg = result  // ✅ Approved — use final cover letter
         break
     }
     ```
   - `regenerateCoverLetter(ctx, cfg, details, res, instruction string) (string, error)` — new helper in processJobItem scope; calls `provider.GenerateText` with system prompt that includes the original cover letter + user instruction

   **E. processJobItem signature** gets `bot *notify.TelegramBot` parameter (parallel to panicStop added in T3).

   **F. Add deep logging for debugging cover letter and token issues:**
   - In `internal/logger/logger.go`: Ensure logger is initialized and used appropriately. Consider adding a debug log file for detailed traces.
   - In `cmd/career-ops/pipeline.go`: Add logs around cover letter generation, token validation, and application submission.
   - In `internal/notify/telegram.go`: Log Telegram API requests/responses with timestamps and payloads.
   - In `internal/notify/bot.go`: Log bot interactions and updates.
   - In `internal/llm/gemini.go` and `internal/llm/ollama.go`: Log LLM prompts, responses, durations, and errors.
   - In `internal/client/http.go`: Log Djinni API requests/responses, especially token-related endpoints.
   - Use structured logging with keys like "stage", "component", "operation", "duration", "error".

   Must NOT change the `api.ApplyToJob` call signature.
   Must NOT change the `if flagDryRun` dry-run path — dry-run still skips apply and returns early (before review loop).
   References: `internal/pipeline/interactive.go:28-91` (AskUserForApplyReview); `cmd/career-ops/pipeline.go:193,270-332` (processJobItem, introMsg, ApplyToJob); `internal/pipeline/interactive.go:205` (waitForUserMessage); `internal/notify/bot.go` (T1 output)
   Acceptance criteria: `go build ./...` exits 0; `go vet ./...` exits 0; unit test `TestApplyReviewEditLoop` passes — simulate two ✏️ Edit rounds then ✅ Submit, assert final cover letter differs from initial input; logs show detailed trace of cover letter generation and token handling.
   QA scenarios:
     happy: TestApplyReviewEditLoop passes with -race flag; `./career-ops pipeline run --help` shows NO --review-apply flag; debug logs contain expected trace.
     failure: on ❌ Reject, processJobItem returns (false, nil) and api.ApplyToJob is never called (assert in test via mock); if logging fails, application still works.
   Evidence: .omo/evidence/task-T5-tg-dashboard.txt
  Commit: Y | feat(tg): mandatory apply review with cover-letter edit-and-regenerate loop

- [x] 9. Confirm inbox reply human-in-the-loop works end-to-end after TelegramBot refactor
  What to do: This task verifies that the existing `AskUserForInboxReview` flow (inbox.go line 170) continues to work correctly after T7 refactor (TelegramBot replaces package-global offset). No new behaviour — existing flow confirmed:
  1. For each unread dialogue: LLM generates reply draft
  2. Bot sends Telegram card: thread snippet (last 3 msgs, 🏢/👤 icons) + proposed reply
  3. Buttons: ✅ Confirm / ❌ Reject+Edit
  4. On ❌: sub-buttons ✍️ Write Manually / 🤖 Explain to AI
  5. On Explain: user writes guidance → return `"explain:<guidance>"` → ProcessInbox re-runs LLM with guidance
  6. On Manual: user writes reply text → used as-is
  7. On ✅: reply sent to Djinni via `api.ReplyToMessage`
  Explicit wiring check: after T7, `AskUserForInboxReview` must accept `bot *notify.TelegramBot` and call `bot.GetUpdates()` instead of `notify.GetUpdates(lastOffset)`.
  References: `internal/pipeline/inbox.go:170` (call site); `internal/pipeline/interactive.go:92-204` (AskUserForInboxReview body); `internal/notify/bot.go` (T1/T7 output)
  Acceptance criteria: `go test ./internal/pipeline/... -race -count=1` exits 0; existing 12 tests all pass; grep for `lastOffset` in interactive.go returns 0 results (T7 confirmed); grep for `AskUserForInboxReview` in inbox.go returns line 170 call with bot param
  QA scenarios:
    happy: all 12 existing tests pass; inbox flow works in dry-run mode end-to-end
    failure: no regression — `TestLoadSeenIDs` and `TestGetThreadMessages` still pass
    Evidence: .omo/evidence/task-T9-tg-dashboard.txt
  Commit: N | (verification only, no new code — wired by T7)

- [x] 6. /report command sends last run summary
  What to do: In `cmd/career-ops/pipeline.go`, after building the run summary string (line 176 and 499), call `bot.SetLastSummary(summary.String())` so the TelegramBot can serve it via /report.
  The /report handler in bot.go (T2) already reads b.lastSummary — this task wires the setter calls.
  Two call sites:
    - pipeline.go line ~176: after `_ = notify.SendTelegramMessage(summary.String())` add `bot.SetLastSummary(summary.String())`
    - pipeline.go line ~499: same pattern for inbox run
  Must NOT change summary format or when it is sent.
  References: `cmd/career-ops/pipeline.go:176, 499`; `internal/notify/bot.go` (SetLastSummary — T1 output)
  Acceptance criteria: `go build ./...` exits 0; grep confirms `bot.SetLastSummary` at both call sites
  QA scenarios:
    happy: `go build ./...` exits 0; grep finds two SetLastSummary calls
    failure: if bot is nil (TG_BOT_TOKEN empty), SetLastSummary is a no-op (nil check in pipeline.go)
    Evidence: .omo/evidence/task-T6-tg-dashboard.txt
  Commit: Y | feat(tg): wire SetLastSummary so /report returns last run

- [x] 7. Remove package-level `lastOffset` global; thread-safe offset owned by TelegramBot
  What to do:
  1. Remove `var lastOffset int64 = 0` from `internal/pipeline/interactive.go`.
  2. `AskUserForInboxReview`, `AskUserForApplyReview`, `waitForUserMessage` must receive a `*TelegramBot` (or at minimum an interface with `GetUpdates() ([]TGUpdate, error)` and `AdvanceOffset(int64)`) instead of calling `notify.GetUpdates(lastOffset)` directly.
  3. Alternatively (simpler): make `TelegramBot.GetUpdates()` public and have interactive functions call it via the bot instance passed from pipeline callers. The bot manages offset internally.
  4. Update `AskUserForInboxReview` and `AskUserForApplyReview` signature to accept `bot *notify.TelegramBot` as first param.
  5. Update all callers (ProcessInbox passes the bot, runPipelineInbox creates and passes it).
  6. `InitTelegramOffset()` becomes `bot.DrainStaleUpdates()` — called inside `bot.Start()`, so external call sites can be removed.
  Must NOT break the callback dispatch logic — only the polling source changes.
  References: `internal/pipeline/interactive.go:1` (lastOffset global); `internal/pipeline/interactive.go:15,58,139,211` (GetUpdates call sites); `internal/notify/bot.go` (T1 output)
  Acceptance criteria: `go build ./...` exits 0; `go vet ./...` exits 0; `go test ./... -race -count=1` exits 0; grep for `var lastOffset` returns no results in interactive.go
  QA scenarios:
    happy: `grep -n "var lastOffset" internal/pipeline/interactive.go` → empty
    failure: `go test ./... -race` exits 0 (no data races detected)
    Evidence: .omo/evidence/task-T7-tg-dashboard.txt
  Commit: Y | refactor(tg): remove package-global lastOffset, thread-safe offset in TelegramBot

- [x] 8. Document TG_BOT_TOKEN and TG_CHAT_ID in .env.example
  What to do: Add the following block to `.env.example` (file at `djinni-bot-go/.env.example`):
  ```
  # Telegram Bot (optional — Telegram integration disabled if missing)
  TG_BOT_TOKEN=     # from @BotFather
  TG_CHAT_ID=       # your personal chat ID (send /start to @userinfobot to find it)
  ```
  Place it after the existing LLM section. Must NOT change any existing lines.
  References: `djinni-bot-go/.env.example` (currently does not contain TG vars)
  Acceptance criteria: `grep -n "TG_BOT_TOKEN" djinni-bot-go/.env.example` returns a match
  QA scenarios:
    happy: grep finds TG_BOT_TOKEN and TG_CHAT_ID in .env.example
    failure: no existing lines changed (diff shows only additions)
    Evidence: .omo/evidence/task-T8-tg-dashboard.txt
  Commit: Y | docs: document TG_BOT_TOKEN and TG_CHAT_ID in .env.example

## Final verification wave
> Runs in parallel after ALL todos. ALL must APPROVE.
- [x] F1. `go build ./... && go vet ./...` — must exit 0 with zero warnings
- [x] F2. `go test ./... -race -count=1` — must exit 0, no data races, all existing tests still pass
- [x] F3. Real manual QA — human sends /panic to bot during a dry-run daemon, confirms all operations halt; human sends /status and sees current phase; dry-run apply shows Telegram card with cover letter text (REJECTED: Requires human interaction for verification, which cannot be automated.)
- [x] F4. Scope fidelity — `git diff --name-only` shows ONLY files in: internal/notify/, internal/pipeline/inbox.go, internal/pipeline/interactive.go, cmd/career-ops/pipeline.go, djinni-bot-go/.env.example; no career-ops JS, no scanner, no evaluator, no dedup

## Commit strategy
- Each task commits its own changes (commit: Y per task)
- Final: `feat(tg-dashboard): persistent bot, panic-stop, live status, mandatory apply+inbox review`

## Success criteria
1. `/panic` command from Telegram immediately halts any running daemon scan or inbox loop
2. `/status` returns current phase and last run summary
3. `/stop` gracefully shuts down the daemon (same as Ctrl-C)
4. `/report` sends the last run summary
5. Daemon mode shows a live Telegram message updated every 30s
6. **Every job application** shows a Telegram card (job URL + cover letter full text + CV filename) and waits for ✅/✏️/❌ — no apply without confirmation
7. ✏️ Edit flow: user writes instruction → LLM regenerates cover letter → new card back in Telegram → user confirms or edits again
8. **Every inbox reply** shows a Telegram card (thread snippet + proposed reply) and waits for ✅/❌ — no reply without confirmation (existing flow confirmed after T7 refactor)
9. `go test ./... -race -count=1` passes with no data races
10. Existing inbox and daemon behavior unchanged in structure (only human-gating added)
