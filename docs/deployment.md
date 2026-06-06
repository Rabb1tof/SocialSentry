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

## Dockerfile (backend)

```dockerfile
# backend/Dockerfile
FROM golang:1.22-alpine AS development
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
CMD ["go", "run", "./cmd/api"]

FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /worker ./cmd/worker

FROM alpine:3.19 AS production
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /api /worker /usr/local/bin/
EXPOSE 8080
```

---

## Dockerfile (frontend)

```dockerfile
# frontend/Dockerfile
FROM node:20-alpine AS development
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
CMD ["npm", "run", "dev", "--", "--host"]

FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine AS production
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

```nginx
# frontend/nginx.conf
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA routing — все пути отдают index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Проксирование API
    location /api/ {
        proxy_pass http://api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # WebSocket
    location /ws {
        proxy_pass http://api:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## Production Docker Compose

```yaml
# docker-compose.prod.yml
version: '3.9'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: ${DB_NAME}
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    restart: unless-stopped

  api:
    image: ghcr.io/yourorg/socialsentry-api:${VERSION}
    depends_on: [postgres, redis]
    environment:
      DATABASE_URL: postgres://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379
    env_file: .env.prod
    restart: unless-stopped
    deploy:
      replicas: 2

  worker:
    image: ghcr.io/yourorg/socialsentry-worker:${VERSION}
    depends_on: [postgres, redis]
    env_file: .env.prod
    restart: unless-stopped

  frontend:
    image: ghcr.io/yourorg/socialsentry-frontend:${VERSION}
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./certs:/etc/nginx/certs:ro
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
```

---

## GitHub Actions CI/CD

```yaml
# .github/workflows/deploy.yml
name: Build and Deploy

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Run tests
        run: cd backend && go test ./...

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Build and push API
        uses: docker/build-push-action@v5
        with:
          context: ./backend
          target: production
          push: true
          tags: ghcr.io/${{ github.repository }}/api:${{ github.sha }}
      - name: Build and push Frontend
        uses: docker/build-push-action@v5
        with:
          context: ./frontend
          target: production
          push: true
          tags: ghcr.io/${{ github.repository }}/frontend:${{ github.sha }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to server
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.SERVER_HOST }}
          username: ${{ secrets.SERVER_USER }}
          key: ${{ secrets.SSH_KEY }}
          script: |
            cd /opt/socialsentry
            VERSION=${{ github.sha }} docker compose -f docker-compose.prod.yml pull
            VERSION=${{ github.sha }} docker compose -f docker-compose.prod.yml up -d
            docker system prune -f
```

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
