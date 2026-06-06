# SocialSentry — Документация

> Платформа автоответов на комментарии и личные сообщения в **Instagram** и **VK**.  
> Мультиаккаунтность · Гибкие триггеры · Система подписок · Роль администратора

---

## Навигация

| Файл | Описание | Читать если... |
|------|---------|----------------|
| [plan.md](./plan.md) | Главный план реализации: API, Engine, подписки, фронтенд, этапы | Хочешь понять что и в каком порядке делать |
| [architecture.md](./architecture.md) | Диаграммы, стек, структура файлов, ключевые решения | Начинаешь работу над проектом |
| [instagram-api.md](./instagram-api.md) | Instagram: OAuth, токены, вебхуки, отправка — **всё проверено** | Работаешь с Instagram интеграцией |
| [vk-api.md](./vk-api.md) | VK: Long Poll, события, отправка, проверка подписки | Работаешь с VK интеграцией |
| [database.md](./database.md) | Полная схема БД, миграции, шифрование, лимиты планов | Работаешь с данными или схемой |
| [deployment.md](./deployment.md) | Docker Compose, Dockerfile, GitHub Actions (CI + release), ngrok | Настраиваешь окружение или деплой |
| [easypanel.md](./easypanel.md) | Деплой в прод на EasyPanel (тянет релизы из GHCR) | Разворачиваешь прод |
| [agents.md](./agents.md) | Правила, соглашения и контекст для AI агентов | Ты AI агент или онбордишь нового разработчика |

---

## Быстрый старт

### Требования

- Go 1.22+
- Node.js 20+
- Docker + Docker Compose
- ngrok (для тестирования Meta Webhooks локально)

### Запуск в dev режиме

```bash
# 1. Клонировать репо
git clone https://github.com/yourorg/socialsentry.git
cd socialsentry

# 2. Создать .env
cp .env.example .env
# Заполнить: DATABASE_URL, REDIS_URL, JWT_SECRET, META_APP_ID, META_APP_SECRET, ...

# 3. Запустить инфраструктуру
docker compose up -d postgres redis

# 4. Применить миграции
cd backend && goose -dir internal/db/migrations postgres "$DATABASE_URL" up

# 5. Запустить API и Worker
go run ./cmd/api &
go run ./cmd/worker &

# 6. Запустить фронтенд
cd ../frontend && npm install && npm run dev

# 7. Для тестирования Meta Webhooks
ngrok http 8080
# Полученный URL → META_CALLBACK_URL в .env
```

---

## Архитектура в двух словах

```
React (Vite)  →  Go API (Gin)  →  PostgreSQL
                     ↓               ↑
                  Redis          Asynq Queue
                     ↓               ↓
               Bot Engine  →  Instagram (graph.facebook.com)
                         ↘  →  VK (Long Poll API)
```

- **Один воркер на аккаунт** — горутина слушает события платформы
- **TriggerMatcher** — находит первый совпавший триггер по приоритету
- **Asynq** — все ответы отправляются асинхронно через очередь
- **Подписка** — без активной подписки воркеры не запускаются, API блокирует изменения

---

## Ключевые факты по Instagram API

> Подробности: [instagram-api.md](./instagram-api.md)

```
✅ Домен:   graph.facebook.com — для ВСЕХ операций
✅ Токен:   Page Access Token (EAAZAjwU...) — один токен для всего
✅ Отправка DM:   POST /{page_id}/messages + access_token в body
✅ Комментарий:   POST /{comment_id}/replies + access_token в body
✅ Диалоги:       GET  /{page_id}/conversations?platform=instagram
✅ Page ID:       получать через GET /me/accounts
✅ Обязательно подписать приложение на страницу при подключении аккаунта:
   POST /{page_id}/subscribed_apps?subscribed_fields=messages,...
⚠️  Отвечать можно только на входящие (не инициировать первым)
⚠️  24-часовое окно для ответа (messaging_type: RESPONSE)
⚠️  200 API вызовов / аккаунт / час
❌  graph.instagram.com — не используется
❌  IGAA токены — не используются
```

---

## Ключевые факты по VK API

> Подробности: [vk-api.md](./vk-api.md)

```
✅ SDK:    github.com/SevereCloud/vksdk/v3
✅ Метод:  Bots Long Poll API (pull, не push)
✅ Токен:  Community Token из настроек группы
✅ Подписка: groups.isMember — быстрая проверка, кэшировать 5 мин
⚠️  20 запросов / секунду на токен
⚠️  random_id в messages.send — уникальный каждый раз
```

---

## Структура аккаунта в БД

```
users
 └── subscriptions          (выдаётся администратором)
 └── connected_accounts     (Instagram или VK)
      └── triggers           (правила автоответов)
           └── trigger_logs  (история срабатываний)
```

---

## Роли пользователей

| Роль | Возможности |
|------|------------|
| `user` | Подключать аккаунты, настраивать триггеры, смотреть логи — при наличии подписки |
| `admin` | Всё то же + управление пользователями, выдача/отзыв подписок, статистика платформы |

Роль задаётся в таблице `users.role`. Изменить роль может только другой `admin` через `PATCH /api/v1/admin/users/:id`.

---

## Планы подписок

| | Basic | Pro | Enterprise |
|-|-------|-----|-----------|
| Аккаунтов | 2 | 10 | ∞ |
| Триггеров / аккаунт | 5 | 50 | ∞ |
| Платформы | VK или IG | VK + IG | VK + IG |
| Логи | 7 дней | 30 дней | 90 дней |

---

## Переменные окружения

| Переменная | Пример | Описание |
|-----------|--------|---------|
| `DATABASE_URL` | `postgres://user:pass@host:5432/db` | PostgreSQL |
| `REDIS_URL` | `redis://localhost:6379` | Redis |
| `JWT_SECRET` | `openssl rand -hex 32` | Подпись JWT |
| `ENCRYPTION_KEY` | `openssl rand -hex 16` | AES ключ для токенов |
| `META_APP_ID` | `1388425723092623` | ID Meta приложения |
| `META_APP_SECRET` | `abc123...` | Секрет Meta приложения |
| `META_WEBHOOK_VERIFY_TOKEN` | `any-random-string` | Верификация вебхука |
| `META_CALLBACK_URL` | `https://domain.com/webhooks/instagram` | URL для Meta |
| `VK_API_VERSION` | `5.199` | Версия VK API |
| `PORT` | `8080` | Порт HTTP сервера |

---

## Этапы разработки

| Фаза | Содержание | Срок |
|------|-----------|------|
| 1 | Auth, БД, базовый UI | 1–2 нед |
| 2 | Подписки + панель админа | 1 нед |
| 3 | Instagram интеграция + ядро движка | 2–3 нед |
| 4 | VK интеграция (адаптер к движку) | 1 нед |
| 5 | Полировка, тесты, деплой | 1–2 нед |

Подробный чеклист каждой фазы: [plan.md → раздел 10](./plan.md)

---

## Технологический стек (кратко)

**Backend:** Go 1.22 · Gin · sqlc + pgx · goose · Asynq · vksdk v3 · zap · graph.facebook.com (Instagram)  
**Storage:** PostgreSQL 16 · Redis 7  
**Frontend:** React 18 · Vite 5 · TypeScript · Tailwind · shadcn/ui · TanStack Query · Zustand  
**Infra:** Docker · GitHub Actions  

Полный стек с обоснованием: [architecture.md → раздел "Технологический стек"](./architecture.md)
