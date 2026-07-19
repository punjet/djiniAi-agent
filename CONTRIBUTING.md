# Contributing & Development Process

## Ветки

| Ветка | Назначение | Кто пушит |
|-------|-----------|-----------|
| `main` | Production. Coolify деплоит отсюда автоматически | Только через merge из `develop` |
| `develop` | Интеграционная. Staging-окружение в Coolify | Merge из feature/* и fix/* |
| `feature/<name>` | Новая функциональность | Создаётся от `develop` |
| `fix/<name>` | Баг-фикс | Создаётся от `develop` (или `main` для хотфикса) |
| `chore/<name>` | Технические правки (deps, ci, docs) | Создаётся от `develop` |

## Жизненный цикл изменения

```
develop → feature/my-feature → develop → main
```

### 1. Начало работы

```bash
git checkout develop
git pull origin develop
git checkout -b feature/inbox-retry-logic
```

### 2. Коммиты

Используем [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(scope):     новая функциональность
fix(scope):      исправление бага
refactor(scope): рефакторинг без изменения поведения
chore(scope):    зависимости, CI, конфиги
docs(scope):     документация
test(scope):     тесты
```

**Scopes** (модули проекта):
- `eval` — оценка вакансий (internal/eval)
- `covergen` — генерация cover letter (internal/covergen)
- `pipeline` — основной pipeline (internal/pipeline)
- `llm` — LLM провайдеры (internal/llm)
- `bot` — Telegram бот
- `cli` — CLI команды (cmd/)
- `docker` — Dockerfile, деплой

**Примеры:**
```bash
git commit -m "feat(eval): добавить retry при невалидном ответе LLM"
git commit -m "fix(pipeline): исправить panic при пустом inbox"
git commit -m "chore(docker): обновить базовый образ до alpine:3.21"
```

### 3. Staging проверка

```bash
git checkout develop
git merge feature/inbox-retry-logic
git push origin develop   # Coolify собирает staging автоматически
```

Проверяешь staging → если всё ок:

### 4. Release в production

```bash
git checkout main
git merge develop
git push origin main      # Coolify деплоит production автоматически
```

### Хотфикс (критический баг в production)

```bash
git checkout main
git checkout -b fix/critical-crash
# ... правишь ...
git commit -m "fix(bot): исправить crash при null session token"
git checkout main
git merge fix/critical-crash
git push origin main
# Потом синхронизируй develop:
git checkout develop
git merge main
git push origin develop
```

---

## Что НИКОГДА не должно попасть в репо

- `.env` файлы с реальными ключами — только `.env.example` с плейсхолдерами
- `*.har` файлы (содержат session tokens)
- Скомпилированные бинарники (`app-binary`, `career-ops`, `djinni-bot-linux`)
- `.omo/` — файлы AI-агентов
- `logs/`, `*.log`

Все они в `.gitignore`. Секреты живут только в Coolify → Environment Variables.

---

## Локальная разработка

### Требования

- Go 1.26+
- `make`

### Первый запуск

```bash
cp .env.example .env
# Заполни .env своими ключами (они не попадут в git)
make build
```

### Полезные команды

```bash
make build          # собрать бинарник
make test           # запустить тесты
make lint           # go vet
make run-evaluate JD="текст вакансии"
make run-test-apply # end-to-end тест pipeline (dry-run)
```

### Перед пушем — всегда проверь

```bash
go build ./...      # должен проходить без ошибок
go vet ./...        # не должно быть предупреждений
```

---

## Coolify деплой

| Событие | Что происходит |
|---------|---------------|
| Push в `develop` | Coolify собирает staging образ |
| Push в `main` | Coolify собирает и деплоит production |
| Build failed | Уведомление в Telegram (если настроено) |

Логи деплоя доступны в Coolify UI → Applications → Deployments.

Переменные окружения настраиваются в Coolify UI → Applications → Environment Variables.  
**Никогда не хардкодь ключи в Dockerfile или коде.**

---

## Структура проекта

```
cmd/career-ops/       — точка входа CLI
internal/
  eval/               — оценка вакансий через LLM
  covergen/           — генерация cover letter и CV
  pipeline/           — основной workflow (inbox, apply)
  llm/                — абстракция над LLM провайдерами
  config/             — загрузка конфига из .env
Dockerfile            — multi-stage build (builder → alpine)
Makefile              — dev-команды
.env.example          — шаблон переменных окружения
```
