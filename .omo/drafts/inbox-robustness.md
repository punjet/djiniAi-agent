---
slug: inbox-robustness
status: awaiting-approval
intent: clear
review_required: false
pending-action: write .omo/plans/inbox-robustness.md
approach: >
  Three-layer fix: (1) immediate idempotency guard using inbox.log +
  conversation-state classifier via full-thread HTML scrape; (2) enrich
  Dialogue with full conversation history; (3) harden the LLM prompt to
  reason over state. No WebSocket integration (undocumented / risky).
  No external DB — use the existing inbox.log file as the seen-IDs registry.
---

# Draft: inbox-robustness

## Components (topology ledger)

| id | outcome | status | evidence path |
|----|---------|--------|---------------|
| A | Idempotency guard: skip dialogID already in inbox.log | active | internal/pipeline/inbox.go:43-44, data/inbox.log |
| B | Full-thread fetcher: GET /my/inbox/{dialogID}/ → parse all messages with timestamps | active | internal/api/inbox.go, api.Dialogue struct |
| C | ConversationState classifier: LLM determines if convo is concluded/active | active | internal/pipeline/inbox.go:61-79 (system prompt) |
| D | Enrich Dialogue struct with thread history | active | internal/api/models.go:6-10 |
| E | Update LLM system prompt to receive full thread + state reasoning | active | internal/pipeline/inbox.go:83-88 |
| F | Telegram review card shows thread context | active | internal/pipeline/interactive.go:94-101 |
| G | Unit tests for idempotency guard and thread parser | active | internal/api/inbox_test.go |

## Open assumptions (announced defaults)

| assumption | adopted default | rationale | reversible? |
|------------|----------------|-----------|-------------|
| Storage for seen dialogIDs | inbox.log (already exists) — parse existing entries on startup | No new DB; inbox.log already has tab-separated dialogID column | yes |
| Thread-fetch strategy | HTML scrape GET /my/inbox/{dialogID}/ | Only confirmed available endpoint; WebSocket schema undocumented | yes |
| State classification | LLM-based classification of thread state (concluded/active/needs-reply) | More robust than keyword matching for natural language | yes |
| Concluded = skip auto-reply | Yes — if state == concluded, skip entire dialogue; notify user via Telegram | Core goal; safe default | yes |
| Timeout for Telegram review loop | 30 minutes (no change in this plan) | Addressing Telegram blocking is separate concern; scope OUT | defer |

## Findings (cited - path:lines)

- `inbox.go:28` — `api.GetUnreadMessages(dc)` fetches only the inbox list page, not thread detail
- `inbox.go:43-44` — `logFile` path is built but **never read before processing** — dialogID not checked for dedup
- `inbox.go:45` — dialogue loop begins immediately; no guard
- `inbox.go:82-134` — LLM loop has no conversation history; only sees `d.Sender` + `d.Message`
- `api/inbox.go:35` — HTML selectors `div.proposal, div.inbox-row, .b-list-jobs__item`; no thread detail fetch
- `api/models.go:6-10` — `Dialogue{ID, Sender, Message}` — no history field
- `interactive.go:94-101` — Telegram card shows sender + last message + proposed reply; no thread context
- `interactive.go:114` — polling loop has no timeout; only ctx.Done() path
- Thread detail available at `GET djinni.co/my/inbox/{dialogID}/` — confirmed HTML endpoint (research finding)
- inbox.log format: `timestamp\tdialogID\tsender\treply` (tab-separated, column 2 = dialogID)

## Decisions (with rationale)

1. **Idempotency via inbox.log read**: On startup of ProcessInbox, read inbox.log and build a `seenIDs map[string]bool`. Skip any dialogue whose ID is already in the map. This is the minimal, zero-dependency fix for the core bug.

2. **Full thread fetch via HTML scrape** of `/my/inbox/{dialogID}/`: Extract all messages (role: recruiter/candidate, text, timestamp) from the thread detail page. This enables the LLM to reason over full context. WebSocket is not used — payload schema is undocumented and adding gorilla/websocket dependency is unwarranted risk.

3. **State classification in system prompt**: Extend the existing system prompt to instruct the LLM to (a) identify thread state: `concluded` (interview scheduled, offer accepted/rejected, candidate already committed), `needs-reply` (open question or greeting), `ambiguous`. Return the state in the JSON output alongside `should_reply`.

4. **Enrich `Dialogue` struct**: Add `Messages []ThreadMessage` where `ThreadMessage{Role, Text, Timestamp}`. The inbox list page still populates `Message` (last message) for backward compat; thread fetch populates `Messages`.

5. **Graceful degradation**: If thread fetch fails (HTML change, network error), fall through to existing single-message behaviour — emit warning in Telegram, don't crash.

6. **Telegram card update**: Include a `Thread (N messages)` summary in the review card so the user sees context before approving.

## Scope IN

- Read inbox.log on startup to build seenIDs set; skip already-processed dialogIDs
- New `api.GetThreadMessages(dc, dialogID)` function — GET + goquery parse of thread page
- `api.ThreadMessage{Role, Text, Timestamp string}` struct
- `api.Dialogue` gets `Messages []ThreadMessage` field
- Updated `ProcessInbox` to call thread fetch per dialogue (with graceful fallback)
- Updated system prompt: state classification + full thread injection
- Updated `InboxReplyResult` JSON to include `"conversation_state": "concluded|needs-reply|ambiguous"`
- Updated Telegram review card to include thread message count and last N messages
- Unit test for seenIDs logic (table-driven, no network)
- Unit test for thread HTML parser (golden HTML fixture)

## Scope OUT (Must NOT have)

- WebSocket integration — undocumented schema, out of scope
- External database (SQLite, Redis, etc.) — inbox.log is sufficient
- Changing the Telegram polling timeout or review flow architecture
- Modifying the job scanning / evaluator pipeline
- Changing the career-ops/ JS codebase
- Automatic reply without Telegram confirmation (human-in-the-loop is preserved)
- Adding new LLM providers or changing the provider interface

## Open questions

None — all decisions are made, defaults adopted and announced.

## Approval gate
status: awaiting-approval
