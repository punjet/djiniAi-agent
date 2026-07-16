# djinni-bot-go: Debug, Test & Stabilization Plan

## TL;DR (For humans)

**Проект сломан** по нескольким причинам одновременно. Вот что нужно сделать:

1. **Исправить `go.mod`** — версия Go 1.26.2 не существует → билд падает
2. **Настроить `.env`** — без Djinni cookies и API ключей ничего не запустится
3. **Заменить хрупкий HTML-парсинг** на более надёжный (или хотя бы покрыть тестами)
4. **Написать интеграционные тесты** с HTTP-фикстурами вместо реальных запросов
5. **Задокументировать все "костыли"** с TODO-комментариями для будущего рефакторинга

**Результат плана**: рабочий `go build`, возможность запустить `career-ops evaluate` без Djinni credentials, чёткая карта всех слабых мест с приоритетами.

---

## Phase 0 — Диагностика и немедленные блокеры

### Задачи

#### TASK-001: Fix go.mod version
**Файл**: `djinni-bot-go/go.mod`
**Проблема**: `go 1.26.2` — несуществующая версия. Сломает `go build` на любом toolchain ≤ 1.23.
**Действие**: Заменить на `go 1.22` (или актуальную установленную версию).
**Проверка**: `cd djinni-bot-go && go build ./...` должен пройти без ошибок.
**Коммит**: `fix(go.mod): downgrade Go version to 1.22 (1.26.2 does not exist)`

#### TASK-002: Create .env.example and document required vars
**Файл**: `djinni-bot-go/.env.example` (создать)
**Проблема**: Нет примера конфига. Пользователь не знает, какие переменные нужны.
**Действие**: Создать `.env.example` с комментариями:
```
# Required for pipeline run / inbox
DJINNI_SESSIONID=your_session_id
DJINNI_CSRFTOKEN=your_csrf_token

# Required for gemini engine (default)
GEMINI_API_KEY=your_api_key
GEMINI_MODEL=gemini-2.5-flash

# Optional: local LLM
OLLAMA_MODEL=llama3.3
OLLAMA_BASE_URL=http://localhost:11434

# Optional: freellmapi
FREELLMAPI_BASE_URL=http://localhost:3001
```
**Коммит**: `docs: add .env.example with required variables`

#### TASK-003: Add startup validation with helpful error messages
**Файл**: `djinni-bot-go/internal/config/config.go`
**Проблема**: Когда не хватает env vars, ошибки нечитаемые.
**Действие**: В `Validate()` добавить конкретные сообщения типа:
```
missing DJINNI_SESSIONID — run `export DJINNI_SESSIONID=<value>` or add to .env
Get your session: browser → djinni.co → F12 → Application → Cookies → djinni_sessionid
```
**Коммит**: `fix(config): improve validation error messages with setup instructions`

---

## Phase 1 — Покрытие тестами (существующих слабых мест)

### Стратегия
- Все тесты используют только `go test ./...` (без сети, без Node.js)
- HTML-фикстуры сохраняются в `internal/extractor/testdata/` как `.html` файлы
- LLM мокается через `llm.Provider` интерфейс (уже существует)

#### TASK-101: Add HTML fixtures for extractor tests
**Файл**: `internal/extractor/testdata/` (создать директорию + файлы)
**Проблема**: Существующий `regex_test.go` скорее всего использует inline HTML. Нужны реальные фрагменты Djinni HTML.
**Действие**: 
- Создать `testdata/dashboard_page.html` — фрагмент реальной страницы дашборда
- Создать `testdata/job_listing.html` — фрагмент поиска
- Создать `testdata/job_details_with_ld.html` — с application/ld+json
- Создать `testdata/job_details_no_ld.html` — fallback parsing
- Создать `testdata/job_no_apply_button.html` — для случая "cant_apply"
**Коммит**: `test(extractor): add HTML fixtures for realistic parsing tests`

#### TASK-102: Expand extractor unit tests
**Файл**: `internal/extractor/regex_test.go`
**Проблема**: Нет тестов на edge cases: нет вакансий, дублирование slug, кириллические заголовки, спецсимволы в компании.
**Действие**: Добавить тест-таблицы для:
- `ExtractJobs` с 0, 1, N результатами + дублями
- `ExtractJobDetails` с/без ld+json, с кириллическими names
- `ExtractDashboardJobs` — когда h2.job-item__position отсутствует
- `cleanDescription` — HTML entities, `&nbsp;`, nested tags
**Коммит**: `test(extractor): add edge case table tests for all extractors`

#### TASK-103: Add mock LLM provider for tests
**Файл**: `internal/llm/mock.go` (создать)
**Проблема**: Нет mock реализации `Provider` интерфейса. Любой тест eval/covergen требует реального LLM или паникует.
**Действие**: 
```go
// MockProvider implements Provider for testing.
type MockProvider struct {
    Response string
    Err      error
    CallCount int
}
func (m *MockProvider) Name() string { return "mock" }
func (m *MockProvider) GenerateText(ctx context.Context, system, user string) (string, error) {
    m.CallCount++
    return m.Response, m.Err
}
```
**Коммит**: `test(llm): add MockProvider for unit tests`

#### TASK-104: Add eval validation tests
**Файл**: `internal/eval/evaluator_test.go` (дополнить)
**Проблема**: Нет тестов на:
- LLM вернул невалидный формат (нет блока A)
- LLM вернул score > 5 или < 0
- Отсутствующий contextDir (все файлы — placeholder)
- Корректный парсинг SCORE_SUMMARY
**Действие**: Добавить table-driven тесты с MockProvider:
- valid full response → EvalResult с корректными полями
- missing Block C → error "LLM returned an invalid..."
- missing SCORE_SUMMARY → error
- score = 5.1 → error
**Коммит**: `test(eval): add table-driven validation and parsing tests`

#### TASK-105: Add pipeline dedup edge case tests  
**Файл**: `internal/pipeline/dedup_test.go` (дополнить)
**Проблема**: Fuzzy matching может давать false positives/negatives. Нет тестов на:
- Один и тот же URL с разными параметрами
- Компания с Unicode/кириллицей
- Роль с 0 significant words (все слова ≤ 3 букв)
- Пустые company/role
**Коммит**: `test(pipeline): add dedup fuzzy match edge case tests`

#### TASK-106: Add config validation tests
**Файл**: `internal/config/config_test.go` (дополнить)
**Проблема**: Нет тестов на частичную конфигурацию (MustLoadPartial), defaults, валидацию.
**Коммит**: `test(config): expand coverage for partial load and validation`

---

## Phase 2 — Интеграционные тесты pipeline с HTTP stub

#### TASK-201: Add httptest server for DjinniClient tests
**Файл**: `internal/client/http_test.go` (дополнить) + `internal/api/jobs_test.go` (дополнить)
**Проблема**: Текущие тесты, вероятно, не делают реальных запросов. Нужно покрыть весь путь от DjinniClient до extractor.
**Действие**: Использовать `net/http/httptest.NewServer()`:
- Сервер отдаёт HTML fixture
- `GetDashboardJobs` → парсит → возвращает правильные Job'ы
- `GetJobDetails` → HTML без `js-inbox-toggle-reply-form` → error "cant_apply"
- `ApplyToJob` → редирект на `?applied=ok` → success
- `ApplyToJob` → редирект без `applied=ok` → error
**Коммит**: `test(api): add httptest-based integration tests for all API functions`

#### TASK-202: Add scanner integration test with stub
**Файл**: `internal/pipeline/scanner_test.go` (дополнить)
**Проблема**: ScanDjinni делает 18+ HTTP запросов (5 pages + 13 searches). Нет теста на полный flow.
**Действие**: httptest сервер, мокающий `/my/dashboard/` и `/jobs/`, проверить что:
- Filter применяется к titles
- Dedup убирает уже виденные
- Warning на ошибку не прерывает остальные запросы
**Коммит**: `test(pipeline): add scanner integration test with httptest stub`

---

## Phase 3 — Документирование и маркировка "костылей"

#### TASK-301: Add TODO/FIXME comments to all crutches
**Файлы**: все затронутые файлы
**Действие**: Добавить стандартизированные комментарии:

```go
// FRAGILE: Djinni HTML-based detection. If Djinni renames "js-inbox-toggle-reply-form"
// CSS class, all jobs will appear as "cant_apply". Track with issue #XX.
// TODO: Switch to API-based apply detection when Djinni exposes one.
```

Список мест:
- `api/jobs.go:86` — js-inbox-toggle-reply-form check
- `api/jobs.go:148` — applied=ok redirect check  
- `extractor/regex.go` — весь файл (FRAGILE: regex HTML parsing)
- `covergen/generator.go:188` — node generate-cover-letter.mjs
- `covergen/generator.go:212` — node generate-cv-html.mjs  
- `cmd/career-ops/evaluate.go:137` — os.Stdout swap (GOROUTINE-UNSAFE)
- `pipeline/scanner.go:118` — hardcoded search queries (NOT configurable)
**Коммит**: `docs: add FRAGILE/TODO markers for all known crutches`

#### TASK-302: Fix goroutine-unsafe os.Stdout swap
**Файл**: `cmd/career-ops/evaluate.go` lines 137-143
**Проблема**: `os.Stdout = os.Stderr` — глобальная мутация, goroutine-unsafe.
**Действие**: Убрать swap, вместо этого передавать `io.Writer` в функции которые пишут в stdout, или использовать отдельный `slog.Logger`.
**Коммит**: `fix(evaluate): remove goroutine-unsafe os.Stdout swap`

#### TASK-303: Extract search queries to config
**Файл**: `internal/pipeline/scanner.go` + `career-ops/portals.yml`
**Проблема**: 13 поисковых запросов захардкожены в Go-коде.
**Действие**: Добавить в `PortalsConfig` поле `SearchQueries []map[string]string`, с fallback на текущий hardcoded список. Если в `portals.yml` указаны queries — использовать их.
**Коммит**: `feat(scanner): make search queries configurable via portals.yml`

---

## Phase 4 — Smoke test и документация запуска

#### TASK-401: Add Makefile targets
**Файл**: `djinni-bot-go/Makefile` (создать)
**Действие**:
```makefile
.PHONY: build test lint run-evaluate

build:
	go build ./cmd/career-ops/

test:
	go test ./... -v -count=1

lint:
	go vet ./...

run-evaluate:
	go run ./cmd/career-ops/ evaluate --jd "$(JD)" --engine gemini
```
**Коммит**: `build: add Makefile with common targets`

#### TASK-402: Add README.md for djinni-bot-go
**Файл**: `djinni-bot-go/README.md` (создать)
**Действие**: 
- Prerequisites (Go 1.22+, Node.js 18+, Playwright)
- Setup steps (copy .env.example, fill credentials)
- How to get Djinni session cookies (browser DevTools)
- Commands with examples
- Architecture diagram (text-art)
- Known limitations / crutches reference
**Коммит**: `docs: add README.md with setup and architecture guide`

---

## Phase 5 — Переход с HTML-парсинга на надёжный разбор ответов (HAR Analysis)

> **ВАЖНЫЙ ВЫВОД из анализа HAR-файлов в `/sup/`:**
> 
> Djinni.co **НЕ имеет JSON REST API** для своей основной функциональности.
> Каждая страница (дашборд, поиск, детали вакансии, инбокс) возвращает **HTML**.
> Все `application/json` ответы в HAR — это **сторонняя аналитика** (Intercom, Sentry),
> не Djinni-эндпоинты. Единственный HTMX-эндпоинт (`/jobs/{id}/similar-jobs/`)
> тоже возвращает **HTML-фрагмент**, а не JSON.
>
> **Что это значит**: Нельзя "заменить HTML-парсинг на API-запросы" в смысле REST JSON.
> Но можно существенно **улучшить надёжность парсинга** следующими способами:

### Стратегия миграции (3 уровня)

**Уровень 1 — Structural data (уже работает, надо расширить):**
Job-страницы содержат `<script type="application/ld+json">` — структурированный JSON прямо в HTML.
Это уже используется в `ExtractJobDetails`. Нужно расширить это на другие поля.

**Уровень 2 — HTML parsing с goquery (надёжнее regex):**
Заменить `regexp` на [`github.com/PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery) —
CSS-selector-based HTML парсер (как jQuery). Намного более устойчив к изменениям верстки.

**Уровень 3 — HTMX API exploration:**
Исследовать возможность вызова HTMX-эндпоинтов напрямую с заголовком `HX-Request: true`.
Это может вернуть более компактные HTML-фрагменты — проще парсить.

---

#### TASK-501: HAR mapping — документировать все Djinni-эндпоинты
**Файл**: `djinni-bot-go/internal/api/ENDPOINTS.md` (создать)
**Действие**: Задокументировать из HAR-файлов:
- `GET /my/dashboard/?page=N` → HTML, job cards в `<h2 class="job-item__position">`
- `GET /jobs/{id}-{slug}/` → HTML с `application/ld+json` блоком
- `GET /jobs/?title={query}&...` → HTML, job list
- `GET /my/inbox/` → HTML, диалоги
- `GET /my/inbox/{id}/` → HTML, одна переписка
- `POST /jobs/{slug}/?ref=for_me` → multipart/form-data → redirect `?applied=ok`
- `POST /my/inbox/{id}/` → multipart/form-data → redirect `?msgsent=ok#last`
- `GET /jobs/{id}/similar-jobs/` (HTMX) → HTML-fragment (HX-Request: true header required)
**Коммит**: `docs(api): add ENDPOINTS.md mapping all Djinni HTTP endpoints from HAR analysis`

#### TASK-502: Add goquery dependency
**Файл**: `djinni-bot-go/go.mod`
**Действие**: `go get github.com/PuerkitoBio/goquery@latest`
**Зачем**: Заменяет `regexp` на CSS-селекторный парсер. Устойчив к переименованию классов если структура DOM сохраняется. Ошибки явные (`.Find` возвращает пустой Selection, не ломается).
**Коммит**: `feat(deps): add goquery for robust HTML parsing`

#### TASK-503: Migrate ExtractDashboardJobs from regex to goquery
**Файл**: `internal/extractor/dashboard.go` (новый файл, оставить `regex.go` без изменений пока)
**Действие**: Реализовать `ExtractDashboardJobsV2(html string)` через goquery:
```go
doc.Find("h2.job-item__position a[href]").Each(func(i int, s *goquery.Selection) {
    href, _ := s.Attr("href")
    // parse /jobs/{id}-{slug}/ from href
    // extract title from text
})
```
Параллельный A/B тест: вызвать оба, сравнить результаты, логировать расхождения.
**Коммит**: `feat(extractor): add goquery-based dashboard job extractor v2`

#### TASK-504: Migrate ExtractJobDetails to ld+json first strategy
**Файл**: `internal/extractor/regex.go` → `internal/extractor/jobdetails.go`
**Действие**: Расширить `application/ld+json` парсинг:
- Из JSON schema: `name` (title), `hiringOrganization.name` (company), `description`
- Fallback на goquery только если ld+json недоступен
- Убрать string-detection `js-inbox-toggle-reply-form` — вместо этого искать `<form[action$="?ref=for_me"]>` через goquery
**Коммит**: `feat(extractor): migrate job details to ld+json primary + goquery fallback`

#### TASK-505: Migrate inbox parsing to goquery
**Файл**: `internal/api/inbox.go`
**Действие**: Заменить regex-парсинг диалогов на goquery:
- `GetUnreadMessages`: `doc.Find(".b-list-jobs__item")` или аналогичный селектор из HAR
- `ReplyToMessage`: оставить POST-запрос как есть, улучшить detection `msgsent=ok` в redirect URL
**Коммит**: `feat(api): migrate inbox HTML parsing to goquery`

#### TASK-506: Improve apply success detection
**Файл**: `internal/api/jobs.go` — `ApplyToJob()`
**Текущая проблема**: Проверяет `strings.Contains(finalURL, "applied=ok")` — работает, но хрупко.
**Действие**:
1. Добавить check HTTP status code (должен быть 200 после redirect)
2. Проверять и `applied=ok` и fallback: `doc.Find(".b-application-status--success")` в теле ответа
3. Добавить явный case для `already applied` — из `alreadyapplied.har` видно что URL меняется
**Коммит**: `fix(api): improve apply success/already-applied detection`

#### TASK-507: Add HTMX endpoint investigation test
**Файл**: `internal/api/jobs_test.go`
**Действие**: Добавить test helper который делает реальный запрос (skip если нет credentials):
```go
// TestHTMXSimilarJobsEndpoint calls /jobs/{id}/similar-jobs/ with HX-Request header
// to verify the HTMX fragment endpoint works and returns HTML, not JSON
```
Это поможет понять можно ли использовать HTMX-эндпоинты для получения компактных данных.
**Коммит**: `test(api): add HTMX endpoint integration test (skipped without credentials)`

---

## Dependency Matrix

```
TASK-001 (go.mod fix) ← независимая, делать первой
TASK-002 (.env.example) ← независимая
TASK-003 (error messages) ← после TASK-001
TASK-101 (fixtures) ← независимая
TASK-102 (extractor tests) ← после TASK-101
TASK-103 (mock LLM) ← независимая
TASK-104 (eval tests) ← после TASK-103
TASK-105 (dedup tests) ← независимая
TASK-106 (config tests) ← независимая
TASK-201 (httptest API) ← после TASK-101
TASK-202 (scanner test) ← после TASK-201
TASK-301 (TODO markers) ← независимая, можно параллельно
TASK-302 (stdout fix) ← независимая
TASK-303 (search queries config) ← после TASK-001
TASK-401 (Makefile) ← после TASK-001
TASK-402 (README) ← последней (описывает итоговое состояние)
TASK-501 (HAR mapping doc) ← независимая
TASK-502 (goquery dep) ← после TASK-001
TASK-503 (dashboard extractor v2) ← после TASK-502 + TASK-101
TASK-504 (jobdetails ld+json) ← после TASK-502
TASK-505 (inbox goquery) ← после TASK-502
TASK-506 (apply detection) ← после TASK-502
TASK-507 (HTMX test) ← независимая
```

## Acceptance Criteria

1. `cd djinni-bot-go && go build ./...` — exit 0 (TASK-001 решает)
2. `go test ./...` — все тесты зелёные (Phase 1 + 2)
3. `go vet ./...` — нет warnings
4. `career-ops evaluate --jd "test" --engine gemini` — работает при наличии GEMINI_API_KEY
5. Все "костыли" помечены в коде с объяснением (TASK-301)
6. Новый разработчик может настроить проект за < 15 минут по README
7. `ExtractDashboardJobsV2` и `ExtractJobDetails` используют goquery, а не только regex
8. `ApplyToJob` проверяет и redirect URL И тело ответа для определения успеха
9. `ENDPOINTS.md` документирует все Djinni эндпоинты из HAR анализа
