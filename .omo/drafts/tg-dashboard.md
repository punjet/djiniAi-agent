---
slug: tg-dashboard
status: awaiting-approval
intent: unclear
pending-action: write .omo/plans/tg-dashboard.md
approach: >
  Extract persistent Telegram polling into a dedicated background goroutine (TelegramBot struct),
  add command routing for /start /status /stop /panic, wire panic-stop flag into all pipeline loops,
  add a live status board that edits a pinned message every N seconds in daemon mode,
  restore AskUserForApplyReview into processJobItem pipeline,
  and make the bot a real interactive control panel without touching the Djinni API layer.
---

# Draft: tg-dashboard

## Components (topology ledger)

| id  | outcome | status | evidence path |
|-----|---------|--------|---------------|
| TG-1 | Persistent polling goroutine (TelegramBot) with graceful shutdown | active | .omo/evidence/task-T1-tg-dashboard.txt |
| TG-2 | Command router: /start /status /stop /panic | active | .omo/evidence/task-T2-tg-dashboard.txt |
| TG-3 | Panic-stop flag propagation into daemon + ProcessInbox + processJobItem | active | .omo/evidence/task-T3-tg-dashboard.txt |
| TG-4 | Live status message pinned during daemon mode (edits every 30s) | active | .omo/evidence/task-T4-tg-dashboard.txt |
| TG-5 | Restore AskUserForApplyReview into processJobItem (was dead code) | active | .omo/evidence/task-T5-tg-dashboard.txt |
| TG-6 | /report command: sends last run summary | active | .omo/evidence/task-T6-tg-dashboard.txt |
| TG-7 | thread-safe lastOffset → TelegramBot method, remove package global | active | .omo/evidence/task-T7-tg-dashboard.txt |
| TG-8 | Document TG_BOT_TOKEN / TG_CHAT_ID in .env.example + config load | active | .omo/evidence/task-T8-tg-dashboard.txt |

## Open assumptions (announced defaults)

| assumption | adopted default | rationale | reversible? |
|-----------|----------------|-----------|-------------|
| Persistent bot approach | goroutine in same process (not a separate service) | no new infra, compatible with CLI invocation | yes — easy to extract |
| Panic-stop mechanism | shared atomic bool flag checked in all loops | simplest safe option, no channels needed | yes |
| Status board | single pinned message edited every 30s (not a separate /dashboard command) | Telegram pins are designed for live status; editing avoids notification spam | yes — 30s interval configurable via constant |
| AskUserForApplyReview | restored as opt-in via --review-apply flag (default false for backward compat) | was dead code, users had 0 approval; back-compat is critical | yes — flag |
| /report content | last run summary (same text as the end-of-run Telegram message) | already computed, zero extra work | yes |
| Bot commands namespace | /start /status /stop /panic /report | industry standard Telegram bot commands; short and memorable | yes |
| TG vars in config | add TG_BOT_TOKEN + TG_CHAT_ID to Config struct + LoadConfig | unifies config system; still reads same env vars | yes |

## Findings (cited - path:lines)

- `internal/notify/telegram.go:1-290` — 6 pure HTTP wrappers; reads TG_BOT_TOKEN/TG_CHAT_ID from os.Getenv on every call; no caching
- `internal/pipeline/interactive.go:1` — `var lastOffset int64 = 0` package global — not thread-safe
- `internal/pipeline/interactive.go:15-22` — InitTelegramOffset() fetches updates once then advances offset
- `internal/pipeline/interactive.go:28-91` — AskUserForApplyReview — DEAD CODE, never called
- `internal/pipeline/interactive.go:91-230` — AskUserForInboxReview / waitForUserMessage — inline polling loops
- `cmd/career-ops/pipeline.go:362-442` — runDaemonMode: infinite loop, time.After(1min), no Telegram interactivity
- `cmd/career-ops/pipeline.go:74-83, 78-82` — signal handler goroutine for SIGINT; only os.Interrupt registered, no SIGTERM
- `cmd/career-ops/pipeline.go:309-326` — processJobItem does NOT call AskUserForApplyReview; auto-applies
- `cmd/career-ops/pipeline.go:176, 499` — SendTelegramMessage for run+inbox summaries
- `internal/pipeline/inbox.go:59-68` — sigChan checked between dialogues
- `internal/config/config.go:13-66` — Config struct; no TG_BOT_TOKEN / TG_CHAT_ID fields
- `.env.example` — does NOT include TG_BOT_TOKEN / TG_CHAT_ID
- daemon uses context.Background() — ctx.Done() branch in daemon loop is dead code

## Decisions (with rationale)

1. **TelegramBot struct in internal/notify** — wraps GetUpdates, owns lastOffset, runs as a goroutine. Keeps notify package cohesive.
2. **Panic stop = atomic bool** — `panicStop *atomic.Bool` passed by pointer through ProcessInbox + processJobItem. When set via /panic command, all loops check it each iteration and bail immediately.
3. **Status message** — maintained as a package-level `statusMsgID int64`; daemon calls `EditMessageText` every 30s with current phase + stats. Simple and testable.
4. **--review-apply flag** — makes apply-review opt-in. Default false means existing daemon users see no behaviour change.
5. **TG_BOT_TOKEN / TG_CHAT_ID in Config** — read via `os.Getenv` in LoadConfig alongside other keys; the notify package functions change to accept token+chatID (or we pass Config to them). Chosen: keep notify functions reading from env (simpler diff); Config just stores them for documentation and startup validation logging.
6. **No webhook** — polling keeps the architecture stateless; no need to expose a port.

## Scope IN

1. `internal/notify/telegram.go` — extract TelegramBot struct with Start/Stop/GetUpdates; keep legacy functions for backward compat
2. `internal/pipeline/interactive.go` — use TelegramBot.GetUpdates instead of bare notify.GetUpdates; remove lastOffset global
3. New command router in notify: handles /start /status /stop /panic /report text commands
4. Panic-stop flag: `internal/notify/bot.go` (new file, or added to telegram.go) exports `panicStop`; ProcessInbox + processJobItem check it
5. Daemon mode: starts TelegramBot goroutine; pins status message; updates every 30s
6. `processJobItem` in pipeline.go: add --review-apply flag; if set, call AskUserForApplyReview before real apply
7. /report command: sends last run summary string
8. `.env.example`: add TG_BOT_TOKEN and TG_CHAT_ID with comment
9. Unit tests: TelegramBot command parsing, panic-stop propagation assertion

## Scope OUT (Must NOT have)

- No Webhook server / HTTP listener
- No external database or Redis
- No changes to Djinni scraping or HTML parsers
- No changes to scanner / evaluator / dedup / career-ops JS
- No UI beyond Telegram messages (no web dashboard)
- No goroutine-per-dialogue (one polling goroutine only)
- No new CLI binary — all changes inside existing `djinni-bot-go`
- No change to ProcessInbox function signature (beyond adding panicStop param — see T3)
- inbox.log format MUST NOT change

## Approval gate
status: awaiting-approval

## Scope update (approved 2026-07-12)

### New decisions from user
- Apply review: MANDATORY (no --review-apply flag; every apply waits for Telegram confirmation)
- Cover letter: shown in full (up to 3000 chars) in TG message
- Edit loop: ✏️ Edit button → user writes instruction → LLM regenerates → new card → repeat until ✅ or ❌
- Inbox reply: existing AskUserForInboxReview flow confirmed as mandatory deliverable (T9)

### Updated task list
- T5: completely rewritten — mandatory apply review with cover letter regeneration loop
  - AskUserForApplyReview new signature: (ctx, bot, company, role, jobURL, score, cvFileName, coverLetter, jobSlug) (approvedCoverLetter string, approved bool, err error)
  - processJobItem gets bot *notify.TelegramBot param
  - regenerateCoverLetter helper in processJobItem scope
  - No --review-apply flag removed from plan
- T9: new task — verify inbox reply human-in-the-loop after T7 refactor (verification only, no new code)

### Updated dependency matrix
- T5 now depends on T1+T2+T3+T7 (not just T1)
- T9 depends on T7
- Wave structure: W1(T1,T8) → W2(T2,T7) → W3(T3) → W4(T4,T5,T6) → W5(T9)

### Status
status: approved (user approved expanded scope 2026-07-12)
