# Comprehensive Logging Audit Report

## 1. Analysis of `internal/logger`
The current logging implementation (`internal/logger/logger.go`) acts as a wrapper around Go's standard `log/slog` library. 

**Strengths:**
- Structured JSON logging via `slog.NewJSONHandler`.
- Ability to attach context loggers (`WithContext`, `FromContext`).
- Dual output formats (Stdout + `djinni-bot.log`).
- Loki integration is functional, sending batched payloads in a background worker.

**Weaknesses & Gaps:**
- **No Trace ID Propagation**: `FromContext` does not automatically extract `X-Trace-Id` (from `internal/trace/trace.go`) or inject it into log records, making request tracing across functions impossible in Loki.
- **Loki Integration Reliability**: The background Loki worker silently discards logs on network errors or marshaling issues (`continue` instead of backoff/retry).
- **`DeepTraceLogger` Isolation**: `LogDeep` writes exclusively to `deep_trace.log` and standard error. It bypasses the standard `slog` output and Loki, making all LLM trace data invisible in centralized Grafana dashboards.
- **Hardcoded Loki Labels**: Loki payloads hardcode `{ "app": "djini-ai-agent", "job": "djinni-bot", "environment": "production" }` instead of allowing environment or module-specific labels.

## 2. Gaps Across Modules

### `internal/api`
- **Silent Failures (Swallowed Errors)**: HTTP handlers (`UploadChatLogHandler`, `UploadInterviewHandler`, `FeedbackHandler`, `ApplicationDetailHandler`, `UpdateStatusHandler`) swallow `db` and parsing errors. They return generic `http.Error` payloads but completely omit `logger.Error()`. These failures are invisible to logging/Grafana.
- **Missing Request/Latency Logging**: Although `traceMiddleware` captures request duration for Prometheus metrics (`httpDuration.WithLabelValues(...)`), it does not log the HTTP request, latency, or status code to `slog`.

### `internal/db`
- **Missing Trace Contexts**: Database operations (e.g., `CreateApplication`, `SaveAgentMemory`) accept `*sql.DB` instead of `context.Context`, making it impossible to correlate slow queries or errors with specific request trace IDs.
- **No Slow Query / Operations Logging**: Insert/Update calls fail silently internally (returning errors to callers that subsequently drop them).

### `internal/client` & `internal/extractor`
- **Zero Logging**: Both modules lack any `slog` or `logger` calls. 
- **Missing Context**: Scraping failures, retries, parsing errors, or payload size limits are not logged as warnings or debug statements, hiding potential flakiness from observability platforms.

### `internal/llm`
- **Invisible to Main Logs**: LLM operations are instrumented through `LoggedProvider` (`internal/llm/logged_engine.go`), but this writes only to `LogDeep` (`deep_trace.log`). Duration, token metrics, and API failures for Gemini/Ollama are not indexed in Loki.

### `internal/pipeline`
- **Mixed Paradigms**: `inbox.go` utilizes both structured logging (`logger.FromContext`) and string appending (`logs = append(logs, ...)`) to track events. The string logs are eventually dumped into a plaintext `inbox.log` bypassing `slog` entirely.
- **Context Loss**: While `loopCtx` injects `dialog_id` into the logger, underlying API calls (`api.ReplyToMessage`, `api.GetUnreadMessages`) do not accept this context, dropping structured identifiers midway.

### `internal/notify`
- **Silent Failures**: Functions in `telegram.go` and `chat.go` execute HTTP requests to the Telegram API but return errors silently. If a Telegram alert fails to send, it is lost unless the caller catches and logs it.

### `cmd/career-ops`
- **Global vs. Contextual Logger Confusion**: There's an equal split between global `logger.Log.Info(...)` and contextual `logger.FromContext(ctx).Info(...)`.
- **Unstructured Key-Values**: Some logs use raw strings instead of structured attributes (e.g., `logger.Log.Info(fmt.Sprintf("..."))`).

## 3. Recommended Improvement Plan
1. **Trace ID Auto-Injection**: Update `internal/logger.FromContext` to automatically extract `X-Trace-Id` from context and append it via `.With("trace_id", traceID)`.
2. **Propagate Context Everywhere**: Refactor `internal/db` and `internal/client` to accept `context.Context` as the first argument, allowing trace IDs to flow down to database and HTTP clients.
3. **HTTP Middleware Logging**: Add a logging statement in `traceMiddleware` to log Method, Path, Status Code, and Duration as structured `slog` fields.
4. **Fix Swallowed API Errors**: Enforce `logger.FromContext(r.Context()).Error(...)` before every `http.Error(...)` call in `internal/api`.
5. **Unify `LogDeep` and `slog`**: Route `DeepTraceLogger` events through the primary `slog` handler with `level=DEBUG` and a specific attribute (e.g., `trace_type=deep`) to ensure LLM usage is visible in Grafana/Loki.
