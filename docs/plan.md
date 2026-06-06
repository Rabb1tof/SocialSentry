# SocialSentry — Полный план реализации

> Платформа автоответов на комментарии и директ в Instagram и VK.  
> Мультиаккаунтность, гибкие триггеры, система подписок с ролью администратора.

---

## Связанные документы

| Файл | Содержимое |
|------|-----------|
| [`instagram-api.md`](./instagram-api.md) | Все эндпоинты Instagram, OAuth flow, токены, лимиты — **проверено на практике** |
| [`vk-api.md`](./vk-api.md) | VK Bots Long Poll API, события, отправка сообщений и комментариев |
| [`database.md`](./database.md) | Полная схема БД, миграции, индексы |
| [`deployment.md`](./deployment.md) | Docker Compose, production-топология, CI/CD |

---

## 1. Архитектура

```
┌─────────────────────────────────────────────────────────────────┐
│                     КЛИЕНТ (React + Vite)                       │
│  Дашборд │ Аккаунты │ Редактор триггеров │ Панель админа        │
└──────────────────────────┬──────────────────────────────────────┘
                           │ REST / WebSocket
┌──────────────────────────▼──────────────────────────────────────┐
│                    API Gateway (Go + Gin)                       │
│  Auth JWT │ Accounts │ Triggers │ Subscriptions │ Admin │ Webhooks│
└──────┬────────────────────┬──────────────────────┬──────────────┘
       │                    │                      │
┌──────▼──────┐   ┌─────────▼────────┐   ┌────────▼───────┐
│ PostgreSQL  │   │   Redis Cache    │   │  Asynq Queue   │
│ (основная   │   │ (сессии,         │   │  (фоновые      │
│  БД)        │   │  rate-limit,     │   │   задачи)      │
└─────────────┘   │  pub/sub)        │   └────────┬───────┘
                  └──────────────────┘            │
                                       ┌──────────▼───────────────┐
                                       │      Bot Engine (Go)     │
                                       │                          │
                                       │  ┌────────────────────┐  │
                                       │  │  Instagram Worker  │  │
                                       │  │  graph.facebook.com│  │
                                       │  │  Page Access Token │  │
                                       │  │  Webhook (Meta)    │  │
                                       │  └────────────────────┘  │
                                       │                          │
                                       │  ┌────────────────────┐  │
                                       │  │    VK Worker       │  │
                                       │  │  vksdk v3          │  │
                                       │  │  Bots Long Poll    │  │
                                       │  └────────────────────┘  │
                                       └──────────────────────────┘
```

### Принцип работы

На каждый подключённый аккаунт запускается горутина-воркер. Воркер слушает входящие события (Meta Webhook / VK Long Poll), сопоставляет текст с триггерами из БД и отправляет ответ. Все воркеры управляются через `context` — при удалении аккаунта или истечении подписки воркер штатно завершается. Триггеры горячо перезагружаются через Redis Pub/Sub без перезапуска воркера.

---

## 2. Стек

| Слой | Технология | Обоснование |
|------|-----------|-------------|
| **Backend** | Go 1.22+ | Горутины = идеальная модель для параллельных воркеров |
| **HTTP** | Gin | Минималистичный, быстрый |
| **SQL** | sqlc + pgx | Типобезопасные запросы |
| **Migrations** | goose | SQL up/down миграции |
| **Queue** | Asynq (Redis) | Retry, мониторинг фоновых задач |
| **VK** | vksdk v3 | Официальный Go SDK |
| **Instagram** | graph.facebook.com + Page Token | Проверено: работает без IGAA токена |
| **БД** | PostgreSQL 16 | JSONB, полнотекстовый поиск |
| **Кэш** | Redis 7 | Сессии, rate-limit, pub/sub, Asynq |
| **Auth** | JWT (access 15m + refresh 7d) + bcrypt | Стандартная схема |
| **Frontend** | React 18 + Vite 5 + TypeScript | |
| **UI** | Tailwind CSS + shadcn/ui | |
| **Стейт** | Zustand + TanStack Query | |
| **WS** | Gorilla WebSocket | Реалтайм уведомления |
| **Деплой** | Docker Compose → Kubernetes | |

---

## 3. Структура монорепозитория

```
socialsentry/
├── backend/
│   ├── cmd/
│   │   ├── api/              # HTTP-сервер
│   │   └── worker/           # Bot Engine
│   ├── internal/
│   │   ├── config/           # Конфиг (env, viper)
│   │   ├── db/
│   │   │   ├── migrations/   # goose SQL-файлы
│   │   │   └── query/        # sqlc-запросы
│   │   ├── domain/           # Бизнес-сущности
│   │   │   ├── user.go
│   │   │   ├── account.go
│   │   │   ├── trigger.go
│   │   │   └── subscription.go
│   │   ├── handler/          # Gin хендлеры
│   │   │   ├── auth.go
│   │   │   ├── accounts.go
│   │   │   ├── triggers.go
│   │   │   ├── subscriptions.go
│   │   │   ├── admin.go
│   │   │   └── webhooks.go
│   │   ├── middleware/
│   │   │   ├── auth.go           # JWT проверка
│   │   │   ├── subscription.go   # Проверка активной подписки
│   │   │   ├── ratelimit.go
│   │   │   └── logger.go
│   │   ├── service/          # Бизнес-логика
│   │   ├── repository/       # Слой данных
│   │   ├── platform/
│   │   │   ├── instagram/    # Meta Graph API клиент
│   │   │   │   ├── client.go
│   │   │   │   ├── messages.go
│   │   │   │   ├── comments.go
│   │   │   │   └── oauth.go
│   │   │   └── vk/           # vksdk обёртка
│   │   │       ├── client.go
│   │   │       ├── messages.go
│   │   │       └── comments.go
│   │   ├── engine/
│   │   │   ├── manager.go    # WorkerManager
│   │   │   ├── worker.go     # AccountWorker
│   │   │   └── matcher.go    # TriggerMatcher
│   │   └── queue/            # Asynq задачи
│   ├── pkg/
│   │   ├── crypto/           # AES-256 шифрование токенов
│   │   ├── jwt/
│   │   └── validator/
│   ├── Dockerfile
│   └── go.mod
│
├── frontend/
│   ├── src/
│   │   ├── api/              # axios + TanStack Query хуки
│   │   ├── components/
│   │   │   ├── ui/           # shadcn компоненты
│   │   │   ├── TriggerEditor/
│   │   │   ├── AccountCard/
│   │   │   └── SubscriptionBanner/
│   │   ├── pages/
│   │   │   ├── auth/
│   │   │   ├── dashboard/
│   │   │   ├── accounts/
│   │   │   ├── triggers/
│   │   │   ├── logs/
│   │   │   └── admin/
│   │   ├── store/            # Zustand
│   │   └── main.tsx
│   ├── vite.config.ts
│   └── package.json
│
├── docs/
│   ├── plan.md               # этот файл
│   ├── instagram-api.md      # ← подробности по Instagram API
│   ├── vk-api.md             # ← подробности по VK API
│   ├── database.md           # ← схема БД
│   └── deployment.md         # ← деплой
│
├── docker-compose.yml
└── .github/workflows/
```

---

## 4. REST API

### Base URL: `/api/v1`

#### Auth
| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/auth/register` | Регистрация |
| POST | `/auth/login` | Логин → access + refresh JWT |
| POST | `/auth/refresh` | Обновить токен |
| POST | `/auth/logout` | Инвалидация |

#### Аккаунты
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/accounts` | Список аккаунтов |
| POST | `/accounts/instagram/connect` | Начать OAuth → вернуть `auth_url` |
| GET | `/accounts/instagram/callback` | OAuth callback от Meta |
| POST | `/accounts/vk/connect` | Подключить VK (community token + group_id) |
| DELETE | `/accounts/:id` | Удалить аккаунт |
| PATCH | `/accounts/:id/status` | Pause / Resume воркера |
| GET | `/accounts/:id/stats` | Статистика аккаунта |

#### Триггеры
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/accounts/:id/triggers` | Список триггеров |
| POST | `/accounts/:id/triggers` | Создать триггер |
| GET | `/accounts/:id/triggers/:tid` | Получить триггер |
| PUT | `/accounts/:id/triggers/:tid` | Обновить триггер |
| DELETE | `/accounts/:id/triggers/:tid` | Удалить |
| PATCH | `/accounts/:id/triggers/:tid/toggle` | Вкл/выкл |
| POST | `/accounts/:id/triggers/:tid/test` | Тест |
| GET | `/accounts/:id/triggers/:tid/logs` | Лог срабатываний |

#### Подписка
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/subscription` | Моя подписка |

#### Администратор
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/admin/users` | Список пользователей |
| GET | `/admin/users/:id` | Детали + аккаунты + подписка |
| PATCH | `/admin/users/:id` | Изменить роль / заблокировать |
| GET | `/admin/subscriptions` | Все подписки |
| POST | `/admin/subscriptions` | Выдать подписку |
| PATCH | `/admin/subscriptions/:id` | Изменить / продлить / деактивировать |
| DELETE | `/admin/subscriptions/:id` | Отозвать |
| GET | `/admin/stats` | Статистика платформы |

#### Вебхуки (публичные)
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/webhooks/instagram` | Верификация Meta (hub.challenge) |
| POST | `/webhooks/instagram` | Входящие события от Meta |
| POST | `/webhooks/vk/:group_id` | VK Callback API (опционально) |

---

## 5. Bot Engine

### WorkerManager

```go
type WorkerManager struct {
    workers map[string]*AccountWorker  // key: account_id
    mu      sync.RWMutex
    repo    repository.AccountRepo
    queue   *asynq.Client
    redis   *redis.Client
}

// При старте — поднять воркеры всех активных аккаунтов с активной подпиской
func (m *WorkerManager) StartAll(ctx context.Context) error

// Запустить воркер для аккаунта
func (m *WorkerManager) StartWorker(ctx context.Context, accountID string) error

// Штатно остановить воркер
func (m *WorkerManager) StopWorker(accountID string) error

// Горячая перезагрузка триггеров без перезапуска (через Redis Pub/Sub)
func (m *WorkerManager) ReloadTriggers(accountID string) error
```

### TriggerMatcher — алгоритм

```
Входящий текст → нормализация (trim, lowercase если не case_sensitive)
    ↓
Загрузить триггеры аккаунта из кэша Redis (TTL 60s) или БД
Отсортировать по priority DESC
    ↓
Для каждого триггера:
  1. is_active = true?
  2. event_type совпадает?
  3. match_mode:
       'all'     → сразу совпадение
       'keyword' → проверить keywords по keywords_mode (contains/exact/starts_with)
       'regex'   → regexp.MatchString
  4. Cooldown: Redis key=trigger_id:sender_id, TTL=cooldown_seconds
  5. Max replies: счётчик в Redis
  6. check_subscription → вызов API платформы
  7. Подставить переменные: {{name}}, {{keyword}}, {{time}}
  8. Поставить задачу в Asynq
  9. Записать в trigger_logs
  10. BREAK — первый совпавший приоритет выигрывает
```

---

## 6. Система подписок

### Middleware

```go
func RequireActiveSubscription() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        sub, err := subService.GetActive(c, userID)

        if err != nil || sub == nil {
            c.JSON(403, gin.H{
                "error":   "subscription_required",
                "message": "Необходима активная подписка",
            })
            c.Abort()
            return
        }

        if sub.ExpiresAt != nil && sub.ExpiresAt.Before(time.Now()) {
            subService.Deactivate(c, sub.ID)
            c.JSON(403, gin.H{"error": "subscription_expired"})
            c.Abort()
            return
        }

        c.Set("subscription", sub)
        c.Next()
    }
}
```

### Что блокируется без подписки
- Подключение аккаунтов
- Создание / изменение триггеров
- Запуск воркеров
- Просмотр логов

### Планы

| Возможность | Basic | Pro | Enterprise |
|------------|-------|-----|-----------|
| Аккаунтов | 2 | 10 | Без лимита |
| Триггеров на аккаунт | 5 | 50 | Без лимита |
| Платформы | VK или IG | VK + IG | VK + IG |
| Логи (глубина) | 7 дней | 30 дней | 90 дней |

Подписка **выдаётся только администратором** вручную. Никакого самостоятельного оформления у пользователя нет.

---

## 7. Фронтенд — страницы

### Публичные
- `/login`
- `/register`

### Пользователь (требует подписку)

**`/dashboard`**
- Карточки: аккаунтов / триггеров / ответов сегодня / ошибок
- Лента событий (WebSocket)
- Баннер статуса подписки

**`/accounts`**
- Карточки аккаунтов: платформа, имя, статус 🟢/🟡/🔴, кол-во триггеров
- Кнопка «Подключить Instagram» → OAuth redirect
- Форма «Подключить VK» → поля Community Token + Group ID
- Pause / Resume / Удалить

**`/accounts/:id/triggers`**
- Таблица с drag-and-drop сортировкой (приоритет)
- Toggle вкл/выкл
- Кнопка «Добавить триггер»

**`/accounts/:id/triggers/new`** и **`/:tid/edit`** — Редактор триггера
- Название
- Тип события: Комментарии / ЛС / Оба
- Режим матчинга: По ключевым словам / На всё / Regex
- Tag-input для ключевых слов
- Режим сравнения: содержит / точное / начинается с
- Регистрозависимость
- Блок действий (для события «Комментарий»):
  - ✅ Ответить публично в треде комментария → поле текста
  - ✅ Отправить личное сообщение комментатору (Private Reply) → поле текста
  - Оба варианта можно включить одновременно
  - Подсказка: «Приватный ответ доступен в течение 7 дней с момента комментария»
- Блок действий (для события «ЛС»):
  - ✅ Ответить в диалог → поле текста
  - Подсказка: «Окно ответа — 24 часа»
- Блок проверки подписки: toggle → два поля
- Кулдаун + макс. ответов
- Подсказка по переменным: `{{name}}`, `{{keyword}}`, `{{time}}`
- Кнопка «Тест»

**`/accounts/:id/logs`**
- Таблица: время, триггер, тип, отправитель, текст, действие, статус
- Фильтры по триггеру / типу / статусу

### Администратор

**`/admin/users`** — таблица пользователей, быстрые действия

**`/admin/users/:id`** — детали + аккаунты + история подписок

**`/admin/subscriptions`** — все подписки, создать / изменить / отозвать

**`/admin/stats`** — графики: пользователи, аккаунты, ответы по дням

---

## 8. Безопасность

| Угроза | Защита |
|--------|--------|
| Перебор паролей | Rate-limit 5 попыток/мин через Redis |
| Кража токенов | Access JWT 15m, Refresh 7d, httpOnly cookie |
| Утечка API-ключей | AES-256-GCM шифрование токенов в БД |
| CSRF | SameSite=Strict, CORS |
| Доступ к чужим данным | Row-level изоляция по user_id из JWT |
| Webhook spoofing (Meta) | Проверка X-Hub-Signature-256 |
| Webhook spoofing (VK) | Проверка secret из тела |
| Превышение лимитов | Token bucket rate-limiter в Redis |

---

## 9. Переменные окружения

```env
# Database
DATABASE_URL=postgres://autoreply:secret@postgres:5432/autoreply

# Redis
REDIS_URL=redis://redis:6379

# Auth
JWT_SECRET=your-super-secret-key
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h

# Encryption (токены платформ в БД)
ENCRYPTION_KEY=32-bytes-hex-key

# Meta / Instagram
META_APP_ID=1388425723092623
META_APP_SECRET=your_app_secret
META_WEBHOOK_VERIFY_TOKEN=your_random_token
META_CALLBACK_URL=https://yourdomain.com/webhooks/instagram

# VK
VK_API_VERSION=5.199

# Server
PORT=8080
ENVIRONMENT=development
```

---

## 10. Поэтапный план разработки

### Фаза 1 — Фундамент (1–2 недели)
- [ ] Монорепо: Go modules, Vite/React, Docker Compose
- [ ] PostgreSQL схема + goose миграции → см. [`database.md`](./database.md)
- [ ] Auth API: register / login / refresh / logout
- [ ] JWT middleware
- [ ] React: Login/Register, роутинг, TanStack Query
- [ ] Layout + навигация дашборда

### Фаза 2 — Подписки и админ (1 неделя)
- [ ] Модель подписки, middleware проверки
- [ ] Admin API: пользователи, выдача подписок
- [ ] Фронт: `/admin/users`, `/admin/subscriptions`
- [ ] Баннер подписки в UI

### Фаза 3 — Instagram интеграция + ядро движка (2–3 недели)
- [ ] CRUD триггеров (платформо-нейтральный API)
- [ ] TriggerMatcher: keyword / all / regex
- [ ] WorkerManager + логирование
- [ ] OAuth flow → см. [`instagram-api.md`](./instagram-api.md)
- [ ] Page подписка (subscribed_apps) при коннекте
- [ ] Webhook endpoint + верификация подписи Meta
- [ ] Instagram воркер: обработка через Asynq
- [ ] Отправка ответа: `POST /v21.0/{page_id}/messages`
- [ ] Ответ на комментарий + Private Reply (DM по comment_id)
- [ ] Rate-limiter (200 запросов/час)
- [ ] Фронт: OAuth кнопка, редактор триггеров, страница аккаунтов

### Фаза 4 — VK интеграция (1 неделя)
- [ ] vksdk v3: Long Poll воркер → см. [`vk-api.md`](./vk-api.md)
- [ ] API подключения VK аккаунта (форма Community Token + group_id)
- [ ] Адаптация существующего движка к VK событиям
- [ ] Ответ на комментарий + отправка ЛС
- [ ] Проверка подписки (groups.isMember) с кэшем Redis
- [ ] Фронт: форма подключения VK

### Фаза 5 — Полировка (1–2 недели)
- [ ] WebSocket реалтайм уведомления
- [ ] Страница логов с фильтрами
- [ ] Статистика + графики
- [ ] Кнопка «Тест триггера»
- [ ] Лимиты по планам подписки
- [ ] Unit-тесты: TriggerMatcher, subscription middleware
- [ ] Integration-тесты: Auth, Webhook
- [ ] Production Docker + GitHub Actions CI → см. [`deployment.md`](./deployment.md)

---

## 11. Стандарт ответов API

```json
// Успех
{
  "data": {},
  "meta": { "page": 1, "per_page": 20, "total": 142 }
}

// Ошибка
{
  "error": "validation_error",
  "message": "Ключевые слова не могут быть пустыми",
  "details": { "keywords": "required" }
}

// Нет подписки
{
  "error": "subscription_required",
  "message": "Необходима активная подписка",
  "subscription_status": "expired"
}
```
