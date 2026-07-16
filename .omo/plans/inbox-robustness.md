# inbox-robustness - Work Plan

## TL;DR (For humans)

**What you'll get:** The bot will stop replying to conversations that are already concluded (like the Svitlana case where the interview was already scheduled). It will also read the full conversation thread before crafting a reply — not just the last message — so the AI has full context of what was said and can detect if the conversation is already resolved.

**Why this approach:** Two load-bearing fixes: (1) a fast idempotency guard using the existing inbox.log file so we never process the same dialogue twice; (2) a full-thread HTML fetch from Djinni's detail page so the LLM sees all messages and can classify whether the conversation is concluded. No new databases, no WebSocket risk — just two targeted additions to the existing code.

**What it will NOT do:** Will not use WebSockets (undocumented API). Will not change how Telegram review works or remove the human confirmation step. Will not touch the job scanning pipeline or career-ops JS code.

**Effort:** Medium
**Risk:** Low — all changes are additive; existing behaviour is preserved as the graceful fallback if the thread fetch fails.
**Decisions to sanity-check:** (1) Using inbox.log as the seen-IDs registry (vs a separate file). (2) Scraping the thread detail page vs WebSocket. Both are safe but worth a glance.

Your next move: **approve** and then run `/start-work`. Full execution detail follows.

---

> TL;DR (machine): Medium effort, Low risk. Adds seenIDs dedup from inbox.log + full-thread HTML fetch + LLM state classification. All additive, fallback-safe.

## Scope

### Must have
- Read `inbox.log` on `ProcessInbox` startup → build `seenIDs map[string]bool` → skip already-processed dialogues
- `api.GetThreadMessages(dc, dialogID) ([]ThreadMessage, error)` — GET `djinni.co/my/inbox/{dialogID}/` + goquery parse of full thread
- `api.ThreadMessage{Role, Text, Timestamp string}` struct in `models.go`
- `api.Dialogue` gets `Messages []ThreadMessage` field (backward compat: `Message` field preserved)
- `ProcessInbox` calls thread fetch per dialogue with graceful fallback (warn + continue with single message)
- Updated LLM system prompt: full thread injected into user prompt; LLM classifies `conversation_state` as `concluded|needs-reply|ambiguous`
- `InboxReplyResult` extended with `ConversationState string` field; if `concluded` → skip reply (log + Telegram notify)
- Telegram review card updated to show thread message count + last 3 messages for context
- Unit test for seenIDs loading from inbox.log (table-driven, no network)
- Unit test for `GetThreadMessages` HTML parser (golden fixture in `testdata/`)

### Must NOT have (guardrails, anti-slop, scope boundaries)
- No WebSocket integration
- No external database (SQLite, Redis, Postgres, etc.)
- No changes to Telegram polling timeout or review flow architecture
- No changes to job scanning pipeline (`scanner.go`, `evaluator.go`, `dedup.go`)
- No changes to `career-ops/` JS codebase
- No auto-reply without Telegram confirmation
- No new LLM providers or provider interface changes
- No `--reset` or destructive operations on `inbox.log`

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: **tests-after** — add unit tests for the two new pure functions; integration is verified by dry-run
- Framework: `go test ./...` with table-driven tests + golden HTML fixture files
- Evidence: `.omo/evidence/task-N-inbox-robustness.txt` (go test output)

## Execution strategy

### Parallel execution waves

**Wave 1 — Data layer (parallel, no dependencies)**
- T1: Add `ThreadMessage` struct and extend `Dialogue` in `models.go`
- T2: Write golden HTML fixture for thread page in `internal/api/testdata/`

**Wave 2 — API layer (depends on W1)**
- T3: Implement `GetThreadMessages()` in `api/inbox.go` + unit test using golden fixture

**Wave 3 — Pipeline layer (depends on W2)**
- T4: Add seenIDs dedup to `ProcessInbox` + unit test for log parsing
- T5: Integrate thread fetch into `ProcessInbox` with graceful fallback
- T6: Update system prompt + `InboxReplyResult` + `concluded` skip logic

**Wave 4 — UX layer (depends on W3)**
- T7: Update Telegram review card to show thread context

**Wave 5 — Final verification (depends on W4)**
- F1–F4: Parallel audit wave

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
|------|-----------|--------|----------------------|
| T1 | — | T3, T4, T5, T6, T7 | T2 |
| T2 | — | T3 | T1 |
| T3 | T1, T2 | T5 | T4 |
| T4 | T1 | — | T5, T6 |
| T5 | T3 | T6, T7 | T4 |
| T6 | T5 | T7 | T4 |
| T7 | T6 | — | — |

## Todos
> Implementation + Test = ONE todo. Never separate.
<!-- APPEND TASK BATCHES BELOW THIS LINE WITH edit/apply_patch - never rewrite the headers above. -->

### Wave 1 (parallel)

- [x] 1. `internal/api/models.go` — Add `ThreadMessage` struct and `Messages []ThreadMessage` to `Dialogue`
  What to do: Add `type ThreadMessage struct { Role string; Text string; Timestamp string }` to models.go. Add `Messages []ThreadMessage \`json:"messages,omitempty"\`` field to `Dialogue` struct. Keep existing `Message string` field for backward compat. Must NOT rename or remove any existing field.
  Parallelization: Wave 1 | Blocked by: nothing | Blocks: T3, T5, T6, T7
  References: `internal/api/models.go:1-23` (full file, 23 lines — simple struct file)
  Acceptance criteria: `go build ./internal/api/...` exits 0; existing `Dialogue{ID, Sender, Message}` literal in `api/inbox.go:73-77` still compiles without change.
  QA scenarios: happy — `go test ./internal/api/... -run TestDialogue` (or `go vet`); failure — introduce a typo in struct tag, confirm `go build` fails. Evidence: `.omo/evidence/task-T1-inbox-robustness.txt`
  Commit: Y | feat(api): add ThreadMessage struct and Messages field to Dialogue

- [x] 2. `internal/api/testdata/thread_page.html` — Create golden HTML fixture for thread page parser
  What to do: Create directory `internal/api/testdata/` if it does not exist. Write a minimal but realistic HTML fixture of a Djinni thread detail page (`GET /my/inbox/{dialogID}/`) that contains at least 3 messages: one from the recruiter, one from the candidate, and one "interview scheduled" closing message. Inspect the real Djinni HTML structure by reading `internal/api/inbox_test.go` for any existing patterns, and refer to the selectors already used in `api/inbox.go:35-78` for naming conventions. The fixture must include: a container element with all messages, per-message elements with role indicator (recruiter/candidate), message text, and a timestamp element. Use plausible but fake data (not real names/companies).
  Parallelization: Wave 1 | Blocked by: nothing | Blocks: T3
  References: `internal/api/inbox.go:35-78` (selector patterns for guidance), `internal/api/inbox_test.go` (existing test patterns)
  Acceptance criteria: File exists at `internal/api/testdata/thread_page.html`; contains at least 3 message elements; `grep -c "message" internal/api/testdata/thread_page.html` returns ≥ 3.
  QA scenarios: happy — file exists and grep count ≥ 3; failure — file missing or empty. Evidence: `.omo/evidence/task-T2-inbox-robustness.txt`
  Commit: Y | test(api): add golden HTML fixture for thread page parser

### Wave 2 (depends on T1+T2)

- [x] 3. `internal/api/inbox.go` — Implement `GetThreadMessages()` + unit test
  What to do: Add function `GetThreadMessages(dc *client.DjinniClient, dialogID string) ([]ThreadMessage, error)` to `internal/api/inbox.go`. It must: (1) GET `https://djinni.co/my/inbox/{dialogID}/` using `dc.Client.R().Get(url)`; (2) parse HTML with goquery; (3) extract all messages from the thread, identifying role (recruiter vs candidate) and timestamp; (4) return `[]ThreadMessage` ordered oldest-first. Must NOT panic on empty thread; return empty slice. Then add `TestGetThreadMessages` in `internal/api/inbox_test.go` that parses the golden fixture from `testdata/thread_page.html` and asserts: (a) at least 3 messages returned; (b) roles are non-empty; (c) the last message text matches the fixture's "interview scheduled" message text. Use `strings.NewReader(html)` to feed the fixture — no real HTTP calls in tests.
  Parallelization: Wave 2 | Blocked by: T1, T2 | Blocks: T5
  References: `internal/api/inbox.go:14-81` (GetUnreadMessages pattern to follow), `internal/api/testdata/thread_page.html` (golden fixture created in T2), `internal/api/models.go` (ThreadMessage struct from T1), `internal/api/inbox_test.go` (existing test file)
  Acceptance criteria: `go test ./internal/api/... -run TestGetThreadMessages -v` exits 0 with PASS; all 3 sub-assertions pass.
  QA scenarios: happy — test passes; failure — intentionally break the goquery selector, confirm test fails with useful message. Evidence: `.omo/evidence/task-T3-inbox-robustness.txt`
  Commit: Y | feat(api): implement GetThreadMessages with full thread HTML parser

### Wave 3 (depends on T3 for T5/T6; T1 for T4; parallel within wave)

- [x] 4. `internal/pipeline/inbox.go` — Add seenIDs dedup guard + unit test
  What to do: At the top of `ProcessInbox` (after `logFile` is assigned at line 43), add a `loadSeenIDs(logFile string) map[string]bool` helper function in the same file. It reads `logFile` line by line, splits on `\t`, and collects column index 1 (dialogID) into the map. If the file doesn't exist, return empty map (not an error). Then in the main loop at line 45, after `for _, d := range dialogues {`, add: `if seenIDs[d.ID] { logs = append(logs, fmt.Sprintf("⏭  Skipping already-processed dialogue %s (%s)", d.ID, d.Sender)); continue }`. Add `TestLoadSeenIDs` in a new file `internal/pipeline/inbox_test.go` (or add to existing if present): write a temp file with 3 lines (2 seen, 1 new), call `loadSeenIDs`, assert the 2 seen IDs are in the map and a random ID is not. Must NOT modify `inbox.log` read logic elsewhere; this is startup-only.
  Parallelization: Wave 3 | Blocked by: T1 | Blocks: nothing | Can parallelize with: T5, T6
  References: `internal/pipeline/inbox.go:43-44` (logFile assignment), `internal/pipeline/inbox.go:45-162` (main loop entry point where guard is inserted), inbox.log format: `timestamp\tdialogID\tsender\treply` (tab col 0=timestamp, 1=dialogID, 2=sender, 3=reply)
  Acceptance criteria: `go test ./internal/pipeline/... -run TestLoadSeenIDs -v` exits 0; a manual dry-run with a pre-seeded inbox.log containing a test dialogID results in log line `⏭  Skipping already-processed dialogue ...` for that ID.
  QA scenarios: happy — test passes, seeded ID skipped; failure — corrupt log line (missing tab) handled gracefully (skip that line, don't panic). Evidence: `.omo/evidence/task-T4-inbox-robustness.txt`
  Commit: Y | fix(pipeline): add seenIDs dedup guard to prevent re-processing concluded dialogues

- [x] 5. `internal/pipeline/inbox.go` — Integrate GetThreadMessages with graceful fallback
  What to do: Inside the `for _, d := range dialogues` loop (after seenIDs check from T4), call `api.GetThreadMessages(dc, d.ID)`. Assign result to `d.Messages`. If error is non-nil, log warning `fmt.Sprintf("⚠️  Thread fetch failed for %s (%s): %v — using last message only", d.ID, d.Sender, err)` and continue with existing single-message behaviour (do NOT break or return). If fetch succeeds, also update `d.Message` to the last message in `d.Messages` for backward compat with existing code paths. Must NOT change the function signature of `ProcessInbox`.
  Parallelization: Wave 3 | Blocked by: T3 | Blocks: T6 | Can parallelize with: T4
  References: `internal/pipeline/inbox.go:27` (ProcessInbox signature), `internal/pipeline/inbox.go:45-162` (dialogue loop), `internal/api/inbox.go` (GetThreadMessages added in T3), `internal/api/models.go` (Dialogue.Messages from T1)
  Acceptance criteria: `go build ./...` exits 0; in dry-run mode, log output contains either thread message count or the fallback warning for each dialogue.
  QA scenarios: happy — build passes + dry-run log shows thread fetch; failure — simulate HTTP error in test by passing invalid dialogID, confirm graceful fallback log emitted. Evidence: `.omo/evidence/task-T5-inbox-robustness.txt`
  Commit: Y | feat(pipeline): integrate full thread fetch into ProcessInbox with graceful fallback

- [x] 6. `internal/pipeline/inbox.go` — Update system prompt + InboxReplyResult + concluded-skip logic
  What to do: (a) Extend `InboxReplyResult` struct to add `ConversationState string \`json:"conversation_state"\`` with values `concluded`, `needs-reply`, `ambiguous`. (b) Update `systemPrompt` string (lines 61-79) to add: a new section "Conversation State" instructing the LLM to analyze the full thread and classify state — `concluded` if interview is scheduled / offer accepted or rejected / candidate already committed; `needs-reply` if there is an open question or greeting needing response; `ambiguous` otherwise. Also update the JSON schema comment in the prompt to include `"conversation_state"` field. (c) Build the `userPrompt` (lines 83-88) to include all thread messages when `d.Messages` is non-empty: format as `--- Thread History ---\n[Role, Timestamp]: Text\n...`; otherwise use the existing single-message format. (d) After JSON parse at line 109, add: `if res.ConversationState == "concluded" { logs = append(logs, fmt.Sprintf("⏭  Conversation with %s is concluded — skipping auto-reply", d.Sender)); notify Telegram with a passive info message; break }`. Must NOT change the JSON output field names `should_reply` and `reply_text`.
  Parallelization: Wave 3 | Blocked by: T5 | Blocks: T7
  References: `internal/pipeline/inbox.go:19-22` (InboxReplyResult struct), `internal/pipeline/inbox.go:61-79` (systemPrompt), `internal/pipeline/inbox.go:83-93` (userPrompt construction), `internal/pipeline/inbox.go:109-113` (JSON parse), `internal/pipeline/inbox.go:118-121` (should_reply check — concluded skip goes here alongside it)
  Acceptance criteria: `go build ./...` exits 0; `go vet ./...` clean; in dry-run with a "concluded" thread fixture, log shows `⏭  Conversation with X is concluded`.
  QA scenarios: happy — build + vet pass; dry-run with concluded fixture skips; failure — break JSON schema in prompt, confirm existing JSON parse error path handles it. Evidence: `.omo/evidence/task-T6-inbox-robustness.txt`
  Commit: Y | feat(pipeline): add conversation state classification and concluded-skip logic

### Wave 4 (depends on T6)

- [x] 7. `internal/pipeline/interactive.go` — Update Telegram review card to show thread context
  What to do: Update `AskUserForInboxReview` signature to accept `messages []api.ThreadMessage` as a new last parameter (add `"djinni-bot-go/internal/api"` import if not present). In the `text` string construction (lines 94-100), add a "Thread" section: if `len(messages) > 0`, append `\n\n📜 *Thread (%d messages):*\n` followed by last min(3, len(messages)) messages formatted as `[Role]: "Text"` (truncate text to 80 chars). If `messages` is nil/empty, omit the section (backward compat). Update the call site in `inbox.go` (line 124) to pass `d.Messages`. Must NOT change any other existing parameter or return type of `AskUserForInboxReview`.
  Parallelization: Wave 4 | Blocked by: T6 | Blocks: nothing
  References: `internal/pipeline/interactive.go:91` (AskUserForInboxReview signature), `internal/pipeline/interactive.go:94-100` (text construction), `internal/pipeline/inbox.go:124` (call site), `internal/api/models.go` (ThreadMessage struct from T1)
  Acceptance criteria: `go build ./...` exits 0; `go vet ./...` clean; updated call site in inbox.go compiles.
  QA scenarios: happy — build + vet pass; failure — pass nil messages slice, confirm no panic (nil check). Evidence: `.omo/evidence/task-T7-inbox-robustness.txt`
  Commit: Y | feat(notify): show thread context in Telegram inbox review card

## Final verification wave
> Runs in parallel after ALL todos. ALL must APPROVE. Surface results and wait for the user's explicit okay before declaring complete.

- [x] F1. Plan compliance audit — run `go build ./...` and `go vet ./...` on the full repo; confirm 0 errors. Confirm `inbox.log` is only read (never truncated or deleted) by grepping all new code for destructive file operations. Evidence: `.omo/evidence/F1-inbox-robustness.txt`
- [x] F2. Code quality review — run `go test ./internal/api/... ./internal/pipeline/... -v -count=1`; all tests pass. Confirm no `panic(` calls in new code. Confirm graceful fallback is present (grep for "fallback" or "Thread fetch failed" log strings). Evidence: `.omo/evidence/F2-inbox-robustness.txt`
- [x] F3. Real dry-run QA — run the inbox pipeline in `--dry-run` mode against the real Djinni account (`cmd/career-ops`); confirm: (a) a previously-logged dialogID is skipped with the `⏭  Skipping already-processed` message; (b) at least one dialogue shows thread context in the Telegram card. Evidence: `.omo/evidence/F3-inbox-robustness.txt` (screenshot or log capture)
- [x] F4. Scope fidelity — grep the diff for any changes to `scanner.go`, `evaluator.go`, `dedup.go`, `career-ops/`; confirm 0 changes. Confirm `inbox.log` is never truncated. Evidence: `.omo/evidence/F4-inbox-robustness.txt`

## Commit strategy
All commits are per-todo (Y flagged above). Final squash optional. Suggested branch: `fix/inbox-robustness`. Merge to main after F1–F4 all APPROVE.

## Success criteria
1. Running `ProcessInbox` twice with the same Djinni account returns `⏭  Skipping already-processed dialogue` for all dialogIDs that were processed in the first run.
2. The Svitlana scenario — a thread where interview has been scheduled and the last unread message is a logistics note — results in `⏭  Conversation with Svitlana is concluded` in the log, with zero auto-reply sent.
3. `go test ./internal/api/... ./internal/pipeline/... -count=1` exits 0.
4. `go build ./...` and `go vet ./...` both exit 0.
