## Task T9: Verify inbox reply human-in-the-loop works after TelegramBot refactor

**Date**: 2026-07-14 20:11:16

### Findings:

1. **Test Suite**: `go test ./internal/pipeline/... -race -count=1` fails to compile due to unrelated test failures in `internal/pipeline/interactive_test.go`:
   ```
   # djinni-bot-go/internal/pipeline [djinni-bot-go/internal/pipeline.test]
   internal/pipeline/interactive_test.go:67:3: not enough arguments in call to AskUserForApplyReview
   	have (context.Context, *notify.TelegramBot, string, string, string, number, string, string, string)
   	want (context.Context, *notify.TelegramBot, string, string, string, float64, string, string, string, int64)
   internal/pipeline/interactive_test.go:104:3: not enough arguments in call to AskUserForApplyReview
   	have (context.Context, *notify.TelegramBot, string, string, string, number, string, string, string)
   	want (context.Context, *notify.TelegramBot, string, string, string, float64, string, string, string, int64)
   FAIL	djinni-bot-go/internal/pipeline [build failed]
   FAIL
   ```

2. **lastOffset check**: `grep -n "lastOffset" internal/pipeline/interactive.go` returns no output (empty), confirming the package-level variable has been removed as part of T7.

3. **AskUserForInboxReview call**: In `internal/pipeline/inbox.go` at line 176:
   ```
               actionText, err := AskUserForInboxReview(ctx, bot, d.Sender, d.Message, res.ReplyText, d.ID, d.Messages)
   ```
   The `bot` parameter (of type `*notify.TelegramBot`) is correctly passed as the second argument.

4. **Function signature**: `AskUserForInboxReview` in `internal/pipeline/interactive.go` line 88 has signature:
   ```
   func AskUserForInboxReview(ctx context.Context, bot *notify.TelegramBot, sender, originalMsg, proposedReply string, dialogueID string, threadMsgs []api.ThreadMessage) (string, error)
   ```
   confirming it expects the bot parameter.

### Conclusion:
The inbox reply human-in-the-loop flow (`AskUserForInboxReview`) has been correctly updated to accept the `*notify.TelegramBot` parameter and the call site passes it. However, the test suite fails to compile due to unrelated issues in `interactive_test.go`. Once those are fixed, the inbox tests should pass.
