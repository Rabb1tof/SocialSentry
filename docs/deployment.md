# Деплой — Docker, CI/CD, Production

---

## Docker Compose (разработка)

```yaml
# docker-compose.yml
version: '3.9'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: socialsentry
      POSTGRES_USER: socialsentry
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U socialsentry"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s

  api:
    build:
      context: ./backend
      target: development
    command: go run ./cmd/api
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    env_file: .env
    ports:
      - "8080:8080"
    volumes:
      - ./backend:/app  # hot reload в dev

  worker:
    build:
      context: ./backend
      target: development
    command: go run ./cmd/worker
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    env_file: .env
    volumes:
      - ./backend:/app

  frontend:
    build:
      context: ./frontend
      target: development
    command: npm run dev
    ports:
      - "3000:3000"
    volumes:
      - ./frontend:/app
      - /app/node_modules

volumes:
  postgres_data:
```

---

## Dockerfiles

The authoritative Dockerfiles live in the repo — don't copy them here, they drift.

- [`backend/Dockerfile`](../backend/Dockerfile) — multi-stage:
  `development` (dev compose), `builder`, `production` (api, `CMD ["api"]`),
  `production-worker` (`CMD ["worker"]`), and `migrate` (goose + migration SQL).
  The `builder` regenerates the gitignored sqlc code (`sqlc generate`) before
  `go build`, so images build from a clean checkout.
- [`frontend/Dockerfile`](../frontend/Dockerfile) — `development`, `builder`,
  `production` (nginx). The production stage serves the SPA **and** reverse-proxies
  `/api`, `/webhooks`, `/ws` to the backend; the upstream is configurable via the
  `API_UPSTREAM` env var, rendered into [`frontend/nginx.conf.template`](../frontend/nginx.conf.template)
  at container start. Single-origin is required (relative `/api/v1` + SameSite=Strict
  refresh cookie).

---

## Production Docker Compose

The production stack is [`docker-compose.prod.yml`](../docker-compose.prod.yml) — it
**pulls** the released images from GHCR (`ghcr.io/rabb1tof/socialsentry/{api,worker,
migrate,frontend}`) instead of building. Services: `postgres`, `redis`, one-shot
`migrate` (goose up), `api`, `worker`, `frontend`. Config comes from `${VAR}`
interpolation — see [`.env.prod.example`](../.env.prod.example).

```bash
cp .env.prod.example .env.prod      # fill secrets, set VERSION (tag without the v)
docker compose --env-file .env.prod -f docker-compose.prod.yml pull
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d
```

**Deploying on EasyPanel → see the step-by-step guide: [`easypanel.md`](./easypanel.md).**

---

## GitHub Actions CI/CD

Two workflows live in `.github/workflows/`:

### `ci.yml` — on every push / PR to `main`

- **backend** job: regenerates the gitignored sqlc code (`sqlc generate`), then
  gofmt check, `go vet`, goose migrations against a Postgres service, `go test
  -race`, builds both binaries, then `golangci-lint`. Postgres 16 + Redis 7 run as
  service containers; integration tests opt in via `TEST_DATABASE_URL` /
  `TEST_REDIS_URL`. (`internal/db/generated` is not committed — CI and the Docker
  builder both run `sqlc generate@v1.31.1`.)
- **frontend** job: `npm ci` + `npm run build` (`tsc --noEmit` + `vite build`) on Node 20.

This is the gate — it does **not** build or push images.

### `release.yml` — on pushing a version tag (`v*.*.*`)

Cutting a release is a one-liner:

```bash
git tag v1.2.0
git push origin v1.2.0     # triggers the workflow
```

What it does:

1. **images** (matrix: `api`, `worker`, `frontend`) — builds the `production`
   stages and pushes to **GHCR** via `docker/build-push-action`. Tags are derived
   from the git tag by `docker/metadata-action`:
   - `1.2.0` (full version, `v` stripped)
   - `1.2` (major.minor)
   - `latest` (only for stable tags; `latest=auto` skips pre-releases)

   The image name is lowercased automatically. `api` is built from the
   `production` target, `worker` from `production-worker` (same image, different
   default command — see the backend Dockerfile).

2. **release** — after all images push, `softprops/action-gh-release` creates a
   GitHub Release named after the tag, with auto-generated notes
   (`generate_release_notes: true`) plus the `docker pull` commands for the three
   images. Tags containing `-` (e.g. `v1.2.0-rc.1`) are marked as pre-releases.

Auth uses the built-in `GITHUB_TOKEN` (workflow grants `contents: write` +
`packages: write`) — **no extra secrets required**. The published packages inherit
the repo's visibility; flip them to public under *Packages → Package settings* if
you want anonymous pulls.

> **Auto-deploy is intentionally not wired.** Promotion to a server (e.g. an
> `appleboy/ssh-action` step running `docker compose -f docker-compose.prod.yml
> pull && up -d`, or a `workflow_dispatch` deploy job) is left out until a deploy
> target + `docker-compose.prod.yml` exist on the host. The published GHCR images
> are ready to `docker pull` by version.

---

## Тестирование вебхуков локально (ngrok)

Для тестирования Meta Webhook нужен публичный URL:

```bash
# Установить ngrok
brew install ngrok  # или скачать с ngrok.com

# Запустить туннель
ngrok http 8080

# Получишь URL вида: https://abc123.ngrok.io
# Использовать как META_CALLBACK_URL=https://abc123.ngrok.io/webhooks/instagram
```

В Meta Developer Console:
- App Dashboard → Webhooks → Add Webhook Product
- Callback URL: `https://abc123.ngrok.io/webhooks/instagram`
- Verify Token: значение из `META_WEBHOOK_VERIFY_TOKEN`
- Subscribe on: `messages`, `comments`

---

## Переменные окружения (.env)

```env
# Database
DATABASE_URL=postgres://socialsentry:secret@postgres:5432/socialsentry
DB_NAME=socialsentry
DB_USER=socialsentry
DB_PASSWORD=secret

# Redis
REDIS_URL=redis://redis:6379
REDIS_PASSWORD=

# Auth
JWT_SECRET=generate-with-openssl-rand-hex-32
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h

# Encryption (для токенов платформ в БД)
# Генерация: openssl rand -hex 16
ENCRYPTION_KEY=your-32-char-hex-key

# Meta / Instagram
META_APP_ID=1388425723092623
META_APP_SECRET=your_app_secret
META_WEBHOOK_VERIFY_TOKEN=your_random_string
META_CALLBACK_URL=https://yourdomain.com/webhooks/instagram

# VK
VK_API_VERSION=5.199

# Server
PORT=8080
ENVIRONMENT=production
LOG_LEVEL=info

# Asynq (мониторинг задач)
ASYNQ_MONITOR_PORT=8081
```

---

## Мониторинг Asynq

Очереди и задачи смотрим через официальный standalone-образ `hibiken/asynqmon` —
он поднимается отдельным сервисом `asynqmon` в `docker-compose.yml` и подключается
напрямую к Redis (никакого кода в `cmd/api`/`cmd/worker` для этого не нужно):

```yaml
# docker-compose.yml
asynqmon:
  image: hibiken/asynqmon:latest
  command: ["--redis-addr=redis:6379"]
  depends_on:
    redis:
      condition: service_healthy
  ports:
    - "${ASYNQ_MONITOR_PORT:-8081}:8080"   # внутри контейнера UI слушает :8080
```

UI доступен по `http://localhost:8081` (корень, не `/monitoring`). Порт настраивается
через `ASYNQ_MONITOR_PORT` в `.env`. Если у Redis задан пароль — добавьте
`--redis-password=...` в `command`.

## Миграции (контейнер `migrate`)

В `docker-compose.yml` есть одноразовый сервис `migrate`: он запускает
`goose -dir internal/db/migrations postgres "<DSN>" up`, применяет все ожидающие
миграции и завершается. Сервисы `api` и `worker` зависят от него через
`condition: service_completed_successfully`, поэтому никогда не стартуют против пустой
или устаревшей схемы. Это не долгоживущий контейнер — после успешного прогона он просто
в статусе `Exited (0)`. Повторный `docker compose up` снова прогоняет `goose up`
(идемпотентно — уже применённые миграции пропускаются).
