# Архитектура, стек и структура проекта

---

## Обзор системы

SocialSentry — платформа для автоматических ответов на комментарии и сообщения в Instagram и VK. Пользователь подключает свои аккаунты, настраивает триггеры по ключевым словам, и бот отвечает за него в реальном времени.

---

## Диаграмма архитектуры

```
┌─────────────────────────────────────────────────────────────────────┐
│                        КЛИЕНТ (React + Vite)                        │
│                                                                     │
│   /dashboard    /accounts    /triggers    /logs    /admin           │
└────────────────────────────┬────────────────────────────────────────┘
                             │ HTTPS REST + WebSocket
┌────────────────────────────▼────────────────────────────────────────┐
│                       API Server (Go + Gin)                         │
│                                                                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │   Auth   │ │ Accounts │ │ Triggers │ │  Admin   │ │ Webhooks │ │
│  │ handler  │ │ handler  │ │ handler  │ │ handler  │ │ handler  │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
│                                                                     │
│  Middleware: JWT auth │ Subscription check │ Rate limit │ Logger   │
└──────┬─────────────────────────┬───────────────────────┬────────────┘
       │                         │                       │
┌──────▼──────┐        ┌─────────▼────────┐    ┌────────▼────────┐
│ PostgreSQL  │        │   Redis          │    │  Asynq Queue    │
│             │        │                  │    │                 │
│ users       │        │ JWT sessions     │    │ instagram_reply │
│ accounts    │        │ rate limiters    │    │ vk_reply        │
│ triggers    │        │ trigger cache    │    │ token_refresh   │
│ sub-s       │        │ sub/status cache │    │ log_cleanup     │
│ logs        │        │ pub/sub reload   │    │                 │
└─────────────┘        └──────────────────┘    └────────┬────────┘
                                                        │
                              ┌─────────────────────────▼───────────┐
                              │          Bot Engine (Go)             │
                              │                                      │
                              │  WorkerManager                       │
                              │  ├── AccountWorker[ig_account_1]     │
                              │  │     └── TriggerMatcher            │
                              │  ├── AccountWorker[ig_account_2]     │
                              │  │     └── TriggerMatcher            │
                              │  ├── AccountWorker[vk_account_1]     │
                              │  │     └── TriggerMatcher            │
                              │  └── AccountWorker[vk_account_N]     │
                              │        └── TriggerMatcher            │
                              └───────────────┬──────────────────────┘
                                              │
                    ┌─────────────────────────┼──────────────────────┐
                    │                         │                      │
          ┌─────────▼──────────┐   ┌──────────▼──────────┐          │
          │   Meta Graph API   │   │      VK API          │          │
          │                    │   │                      │          │
          │ graph.facebook.com │   │  api.vk.com          │          │
          │ Page Access Token  │   │  Community Token     │          │
          │                    │   │  Bots Long Poll      │          │
          │ → POST /messages   │   │  → messages.send     │          │
          │ → POST /replies    │   │  → wall.createComment│          │
          └────────────────────┘   └──────────────────────┘          │
                    ▲                         ▲                      │
                    │                         │                      │
          ┌─────────┴──────────┐              │              ┌───────┘
          │  Meta Webhooks     │              │              │
          │  (push → наш сервер│              │              │
          │   /webhooks/instagram)            │    Long Poll │
          └────────────────────┘              │    (pull)    │
                                              └─────────────┘
```

---

## Потоки данных

### Instagram: входящее DM

```
Пользователь пишет в Instagram директ
    → Meta Webhook: POST /webhooks/instagram
        → Проверка X-Hub-Signature-256
        → Немедленный HTTP 200
        → Asynq: enqueue task "process_instagram_dm"
            → Worker: загрузить аккаунт из БД
            → TriggerMatcher: найти совпадение
            → Проверка cooldown (Redis)
            → Проверка подписки (опционально)
            → POST graph.facebook.com/{page_id}/messages
            → Запись в trigger_logs
```

### VK: входящее сообщение

```
Пользователь пишет в ЛС сообщества VK
    → VK Bots Long Poll: событие MessageNew
        → AccountWorker получает событие
        → TriggerMatcher: найти совпадение
        → Проверка cooldown (Redis)
        → Проверка подписки: groups.isMember (кэш Redis 5 мин)
        → vk.MessagesSend
        → Запись в trigger_logs
```

### Горячая перезагрузка триггеров

```
Пользователь изменяет триггер в UI
    → PUT /api/v1/accounts/:id/triggers/:tid
        → Обновление в PostgreSQL
        → Redis PUBLISH "triggers:reload:{account_id}"
            → WorkerManager подписан на канал
            → Инвалидация кэша триггеров этого аккаунта
            → Следующий запрос матчера загрузит свежие триггеры
```

---

## Технологический стек

### Backend

| Компонент | Технология | Версия | Назначение |
|-----------|-----------|--------|-----------|
| Язык | Go | 1.22+ | Основной язык |
| HTTP framework | Gin | v1.9+ | REST API |
| SQL | sqlc + pgx/v5 | latest | Типобезопасные запросы |
| Миграции | goose | v3 | SQL up/down |
| Task queue | Asynq | v0.24+ | Фоновые задачи, retry |
| VK SDK | vksdk/v3 | v3 | Long Poll, все события, отправка |
| Config | viper | v1 | ENV + YAML конфиг |
| Логирование | zap | v1 | Структурированные логи |
| WebSocket | gorilla/websocket | v1 | Реалтайм события в UI |

### Базы данных и инфра

| Компонент | Технология | Версия | Назначение |
|-----------|-----------|--------|-----------|
| Основная БД | PostgreSQL | 16 | Все данные приложения |
| Кэш / очереди | Redis | 7 | Сессии, rate-limit, Asynq, Pub/Sub |
| Контейнеры | Docker + Compose | latest | Dev и prod окружения |
| CI/CD | GitHub Actions | — | Тесты, сборка, деплой |

### Frontend

| Компонент | Технология | Версия | Назначение |
|-----------|-----------|--------|-----------|
| Framework | React | 18 | UI |
| Сборщик | Vite | 5 | Быстрая разработка |
| Язык | TypeScript | 5 | Типизация |
| UI Kit | shadcn/ui | latest | Компоненты |
| Стили | Tailwind CSS | v3 | Утилитарные стили |
| Серверный стейт | TanStack Query | v5 | Кэш, инвалидация, запросы |
| Клиентский стейт | Zustand | v4 | Глобальный стейт |
| Роутинг | React Router | v6 | SPA навигация |
| HTTP клиент | axios | v1 | API запросы |
| Формы | React Hook Form + zod | latest | Валидация |
| Графики | Recharts | v2 | Статистика |
| DnD | @dnd-kit | v6 | Drag-and-drop триггеров |

### Внешние API

| Сервис | Метод | Токен |
|--------|-------|-------|
| Instagram (DM + комментарии) | `graph.facebook.com` v21+ | Page Access Token (`EAAZAjwU...`) |
| Instagram события | Meta Webhooks (push) → `/webhooks/instagram` | Верификация X-Hub-Signature-256 |
| VK | Bots Long Poll API v5.199 | Community Token |

---

## Структура проекта

```
socialsentry/
│
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   │   └── main.go              # Точка входа HTTP-сервера
│   │   └── worker/
│   │       └── main.go              # Точка входа Bot Engine
│   │
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go            # Загрузка конфига из ENV
│   │   │
│   │   ├── db/
│   │   │   ├── migrations/          # goose SQL-файлы
│   │   │   │   ├── 001_users.sql
│   │   │   │   ├── 002_subscriptions.sql
│   │   │   │   ├── 003_accounts.sql
│   │   │   │   ├── 004_triggers.sql
│   │   │   │   ├── 005_trigger_logs.sql
│   │   │   │   └── 006_refresh_tokens.sql
│   │   │   ├── query/               # SQL-запросы для sqlc
│   │   │   │   ├── users.sql
│   │   │   │   ├── accounts.sql
│   │   │   │   ├── triggers.sql
│   │   │   │   └── subscriptions.sql
│   │   │   └── generated/           # Авто-генерация sqlc (не редактировать)
│   │   │
│   │   ├── domain/                  # Бизнес-сущности (чистые структуры)
│   │   │   ├── user.go
│   │   │   ├── account.go
│   │   │   ├── trigger.go
│   │   │   ├── subscription.go
│   │   │   └── log.go
│   │   │
│   │   ├── handler/                 # HTTP-хендлеры (только HTTP-слой)
│   │   │   ├── auth.go
│   │   │   ├── accounts.go
│   │   │   ├── triggers.go
│   │   │   ├── subscriptions.go
│   │   │   ├── admin.go
│   │   │   ├── webhooks.go
│   │   │   └── ws.go                # WebSocket
│   │   │
│   │   ├── middleware/
│   │   │   ├── auth.go              # JWT извлечение и проверка
│   │   │   ├── subscription.go      # Проверка активной подписки
│   │   │   ├── admin.go             # Проверка роли admin
│   │   │   ├── ratelimit.go         # Redis-based rate limiter
│   │   │   └── logger.go            # Request logging (zap)
│   │   │
│   │   ├── service/                 # Бизнес-логика
│   │   │   ├── auth.go
│   │   │   ├── account.go
│   │   │   ├── trigger.go
│   │   │   ├── subscription.go
│   │   │   └── admin.go
│   │   │
│   │   ├── repository/              # Слой данных (интерфейсы + реализация)
│   │   │   ├── user.go
│   │   │   ├── account.go
│   │   │   ├── trigger.go
│   │   │   ├── subscription.go
│   │   │   └── log.go
│   │   │
│   │   ├── platform/
│   │   │   ├── instagram/
│   │   │   │   ├── client.go        # HTTP-клиент graph.facebook.com
│   │   │   │   ├── oauth.go         # OAuth flow, обмен токенов
│   │   │   │   ├── messages.go      # Отправка DM
│   │   │   │   ├── comments.go      # Ответы на комментарии
│   │   │   │   ├── webhook.go       # Парсинг и верификация вебхуков
│   │   │   │   └── ratelimit.go     # 200 rph rate limiter
│   │   │   │
│   │   │   └── vk/
│   │   │       ├── client.go        # vksdk обёртка
│   │   │       ├── worker.go        # Long Poll горутина
│   │   │       ├── messages.go      # messages.send
│   │   │       ├── comments.go      # wall.createComment
│   │   │       └── subscription.go  # groups.isMember + Redis cache
│   │   │
│   │   ├── engine/
│   │   │   ├── manager.go           # WorkerManager: старт/стоп/reload
│   │   │   ├── worker.go            # AccountWorker: горутина на аккаунт
│   │   │   └── matcher.go           # TriggerMatcher: алгоритм матчинга
│   │   │
│   │   └── queue/
│   │       ├── client.go            # Asynq клиент
│   │       ├── tasks.go             # Определения задач
│   │       └── handlers/
│   │           ├── instagram.go     # Обработчик IG событий
│   │           ├── vk.go            # Обработчик VK событий
│   │           └── maintenance.go   # Ротация логов, обновление токенов
│   │
│   ├── pkg/
│   │   ├── crypto/
│   │   │   └── aes.go               # AES-256-GCM шифрование токенов
│   │   ├── jwt/
│   │   │   └── jwt.go               # Генерация и валидация JWT
│   │   └── validator/
│   │       └── validator.go         # Кастомные правила валидации
│   │
│   ├── Dockerfile
│   └── go.mod
│
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   │   ├── client.ts            # axios instance + interceptors
│   │   │   ├── auth.ts              # хуки: useLogin, useRegister, useLogout
│   │   │   ├── accounts.ts          # хуки: useAccounts, useConnectIG, useConnectVK
│   │   │   ├── triggers.ts          # хуки: useTriggers, useCreateTrigger и тд
│   │   │   ├── subscriptions.ts     # хуки: useMySubscription
│   │   │   └── admin.ts             # хуки: useAdminUsers, useGrantSubscription
│   │   │
│   │   ├── components/
│   │   │   ├── ui/                  # shadcn компоненты (не редактировать)
│   │   │   ├── layout/
│   │   │   │   ├── Sidebar.tsx
│   │   │   │   ├── Header.tsx
│   │   │   │   └── AdminLayout.tsx
│   │   │   ├── AccountCard.tsx      # Карточка подключённого аккаунта
│   │   │   ├── TriggerEditor/
│   │   │   │   ├── index.tsx        # Главная форма редактора
│   │   │   │   ├── KeywordInput.tsx # Tag-input для ключевых слов
│   │   │   │   ├── ActionBlock.tsx  # Блок действий (ответ/ЛС)
│   │   │   │   └── SubCheckBlock.tsx# Блок проверки подписки
│   │   │   ├── TriggerList.tsx      # Список с DnD сортировкой
│   │   │   ├── SubscriptionBanner.tsx
│   │   │   └── StatsCard.tsx
│   │   │
│   │   ├── pages/
│   │   │   ├── auth/
│   │   │   │   ├── Login.tsx
│   │   │   │   └── Register.tsx
│   │   │   ├── dashboard/
│   │   │   │   └── Dashboard.tsx
│   │   │   ├── accounts/
│   │   │   │   ├── Accounts.tsx
│   │   │   │   └── ConnectAccount.tsx
│   │   │   ├── triggers/
│   │   │   │   ├── Triggers.tsx
│   │   │   │   ├── TriggerNew.tsx
│   │   │   │   ├── TriggerEdit.tsx
│   │   │   │   └── TriggerLogs.tsx
│   │   │   └── admin/
│   │   │       ├── AdminUsers.tsx
│   │   │       ├── AdminUserDetail.tsx
│   │   │       ├── AdminSubscriptions.tsx
│   │   │       └── AdminStats.tsx
│   │   │
│   │   ├── store/
│   │   │   ├── auth.ts              # Zustand: текущий пользователь, токены
│   │   │   └── ws.ts                # Zustand: WebSocket соединение
│   │   │
│   │   ├── router.tsx               # React Router конфиг + защищённые маршруты
│   │   ├── App.tsx
│   │   └── main.tsx
│   │
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   └── package.json
│
├── docs/
│   ├── README.md                    # Индекс документации
│   ├── plan.md                      # Главный план реализации
│   ├── architecture.md              # Этот файл
│   ├── instagram-api.md             # Instagram API (проверено на практике)
│   ├── vk-api.md                    # VK API
│   ├── database.md                  # Схема БД
│   ├── deployment.md                # Деплой
│   └── agents.md                    # Инструкции для AI агентов
│
├── docker-compose.yml               # Dev окружение
├── docker-compose.prod.yml          # Production
├── .env.example
└── .github/
    └── workflows/
        └── deploy.yml
```

---

## Ключевые архитектурные решения

### Почему Go, а не .NET

Горутины — нативная модель конкурентности. Для 50 аккаунтов = 50 воркеров, каждый слушает свою платформу. Минимальный overhead, компилируется в один бинарник, отличная производительность для I/O-bound задач.

### Почему два бинарника (api + worker)

`cmd/api` — HTTP-сервер. `cmd/worker` — Bot Engine. Разделение позволяет масштабировать независимо: нужно больше воркеров — добавляем реплики worker-сервиса без затрагивания API.

### Почему Asynq, а не горутины напрямую

Webhook от Meta приходит и ждёт ответа максимум 20 секунд. Нельзя делать всё синхронно. Asynq даёт: немедленный ответ 200 → задача в Redis → retry при ошибке → мониторинг очереди.

### Почему graph.facebook.com, а не graph.instagram.com

Проверено на практике: `graph.facebook.com + Page Access Token-токен через полный Instagram OAuth. `graph.facebook.com` с Page Access Token даёт те же возможности и получается проще — через `/me/accounts` после стандартного Facebook OAuth.

### Горячая перезагрузка триггеров

При изменении триггера → Redis `PUBLISH triggers:reload:{account_id}` → WorkerManager инвалидирует кэш. Следующий матчинг загрузит свежие данные. Воркер не перезапускается, соединение с платформой не разрывается.

### Шифрование токенов

Токены платформ в БД хранятся зашифрованными AES-256-GCM. Ключ шифрования — только в ENV, никогда не в коде и не в БД. Компрометация БД не даёт доступ к токенам.
