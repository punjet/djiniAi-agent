# Logging Architecture Improvement & Expansion Plan

## Executive Summary
This document establishes the architecture plan for standardizing application logging, introducing request context propagation, eliminating swallowed API errors, and shipping container logs to Grafana Loki (`afxvdtxdynrb4b`).

---

## 1. Current State Summary

### 1.1 Application Logger Setup (`internal/logger/logger.go`)
- The application uses Go's standard `log/slog` structured logging package configured to output structured JSON to stdout.
- `logger.Init()` configures `slog.JSONHandler` with customized `timeKey: "timestamp"` and `levelKey: "level"`, mapping levels to upper-case strings (`INFO`, `ERROR`, `WARN`, `DEBUG`).
- Core services (such as LLM calls via `logged_engine.go` and server startup in `cmd/career-ops/main.go`) emit well-structured JSON fields (`duration_ms`, `tokens`, `model`, `component`).

### 1.2 Infrastructure Log Pipeline
- **Coolify Deployment**: Applications run inside Docker containers managed by Coolify. stdout/stderr streams are captured by the Docker engine.
- **Grafana Loki Instance**: Loki datasource (`afxvdtxdynrb4b`) is configured in Grafana, but currently contains **no log streams**. Logs currently remain local to container stdout.

---

## 2. Identified Gaps

### 2.1 Swallowed Errors in HTTP Handlers (`internal/api`)
Several API handlers catch database or processing errors, output generic HTTP 500 status codes, but **fail to log the underlying Go error**:
- `FeedbackHandler` (`internal/api/feedback.go`): Silently discards DB insertion errors (`http.Error(w, err.Error(), 500)` or generic strings) without emitting an `slog.ErrorContext` log entry.
- `UploadChatLogHandler` (`internal/api/chatlog.go`): Returns HTTP 400/500 responses without logging contextual details (user ID, payload size, DB error).
- `InterviewHandler` & `JobsHandler`: Lack consistent error field logging (`slog.Any("error", err)`).

### 2.2 Lack of Request Correlation IDs
- Incoming HTTP requests do not receive or generate a `X-Request-ID` or `trace_id`.
- Downstream database operations, LLM pipeline executions, and notification events cannot be traced back to the originating HTTP request or background daemon job.

### 2.3 Legacy stdlib `log` Package Usage
- CLI commands (`cmd/career-ops/*.go`) and pipeline utilities (`internal/pipeline/interactive.go`, `internal/pipeline/inbox.go`) still import Go's legacy `log` package (`log.Printf`, `log.Fatalf`).
- Legacy logs bypass `slog.JSONHandler`, emitting unstructured text strings into Docker stdout, making parsing in Loki unreliable.

### 2.4 Unconfigured Loki Log Collector
- Docker containers in Coolify stream to the default `json-file` Docker logging driver.
- Neither a Grafana Alloy/Promtail daemonset/sidecar nor the Docker Loki log driver is configured to forward container stdout to Loki (`afxvdtxdynrb4b`).

---

## 3. Actionable Improvement Steps

### Step 1: Application-Wide `slog` Standardization
1. **Deprecate `log.Printf`**: Replace all standard `log.*` imports in `cmd/career-ops/` and `internal/pipeline/` with `slog.Info`, `slog.Error`, or `slog.Debug`.
2. **Mandate Contextual Logging**: Enforce `slog.InfoContext(ctx, ...)` and `slog.ErrorContext(ctx, ...)` across all internal packages to allow context values (like correlation IDs) to be automatically extracted into log fields.

### Step 2: Request Context Propagation & Middleware
1. **HTTP Correlation Middleware**:
   Create a middleware in `internal/api/middleware.go`:
   - Extract `X-Request-ID` from incoming HTTP headers or generate a UUIDv4 if missing.
   - Inject `request_id` into `r.Context()`.
   - Set `X-Request-ID` header on the response.
2. **Context-Aware `slog.Handler` Wrapper**:
   Extend `internal/logger/logger.go` with a custom `slog.Handler` wrapper that automatically extracts `request_id`, `user_id`, or `job_id` from Go's `context.Context` and attaches them as top-level JSON fields.

### Step 3: API Error Logging & Recovery Middleware
1. **Centralized Error & Recovery Middleware**:
   - Intercept panics and unhandled HTTP errors in `internal/api`.
   - Automatically log HTTP status code, duration, method, path, remote IP, and error stack trace (for 5xx) with `slog.ErrorContext`.
2. **Explicit Handler Logging**:
   Update `FeedbackHandler`, `UploadChatLogHandler`, `InterviewHandler`, and `JobsHandler` to log explicit error context before returning error responses:
   ```go
   slog.ErrorContext(ctx, "failed to save feedback",
       slog.String("user_id", req.UserID),
       slog.Any("error", err),
   )
   ```

### Step 4: Infrastructure Log Shipping to Grafana Loki
Choose and implement one of the following shipping drivers depending on Coolify server permissions:

- **Option A: Promtail / Grafana Alloy Container (Recommended)**
  Deploy Grafana Alloy / Promtail as a Coolify service mounted to `/var/lib/docker/containers` (read-only). Configure it to scrape container log files, parse JSON stdout, and forward to the Loki instance (`afxvdtxdynrb4b`).
  
- **Option B: Docker Loki Logging Driver**
  Configure Coolify server daemon or Docker Compose service with the `loki` log driver:
  ```yaml
  logging:
    driver: loki
    options:
      loki-url: "http://<loki-host>:3100/loki/api/v1/push"
      loki-external-labels: container_name={{.Name}},app=djini-agent
  ```

---

## 4. Implementation Milestones

```
+-------------------------------------------------------------------------------+
| PHASE 1: Code-Level Error & Middleware Hardening                             |
| - Implement Request ID & Recovery Middleware in internal/api                 |
| - Update FeedbackHandler, UploadChatLogHandler, etc. to log 500 DB errors     |
| - Replace legacy stdlib `log` calls with `slog` across cmd/ and internal/     |
+-------------------------------------------------------------------------------+
                                       |
                                       v
+-------------------------------------------------------------------------------+
| PHASE 2: Context Propagation & Custom Handler                                 |
| - Implement Context-aware slog Handler wrapper extracting request_id          |
| - Propagate ctx through DB, LLM, and Notify calls                             |
| - Verify stdout JSON outputs contain correlated request_id fields             |
+-------------------------------------------------------------------------------+
                                       |
                                       v
+-------------------------------------------------------------------------------+
| PHASE 3: Infrastructure Log Forwarding & Verification                         |
| - Deploy Promtail / Grafana Alloy container or configure Docker Loki driver   |
| - Verify active log streams in Grafana Loki (datasource `afxvdtxdynrb4b`)     |
| - Create basic Grafana dashboard / alert rules for error rate spikes (5xx)    |
+-------------------------------------------------------------------------------+
```

### Milestone Timeline Summary
- **Milestone 1**: HTTP Middleware & API Error Logging Audit (1-2 Days)
- **Milestone 2**: Application-wide `slog` Refactor & Context Tracing (2 Days)
- **Milestone 3**: Grafana Loki Log Forwarder Configuration & Dashboard Setup (1 Day)
