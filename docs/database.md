# База данных — Схема и миграции

> PostgreSQL 16. Миграции через goose.  
> Все токены платформ хранятся в зашифрованном виде (AES-256-GCM).

---

## Таблицы

### users

```sql
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,                    -- bcrypt hash
    role        TEXT NOT NULL DEFAULT 'user',     -- 'user' | 'admin'
    is_blocked  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);
```

### subscriptions

Подписка **выдаётся только администратором**. Самооформление отсутствует.

```sql
CREATE TABLE subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan        TEXT NOT NULL DEFAULT 'basic',    -- 'basic' | 'pro' | 'enterprise'
    is_active   BOOLEAN NOT NULL DEFAULT true,
    starts_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,                      -- NULL = бессрочная
    note        TEXT,                             -- комментарий администратора
    granted_by  UUID REFERENCES users(id),        -- admin id
    created_at  TIMESTAMPTZ DEFAULT now()
);
```

### connected_accounts

```sql
CREATE TABLE connected_accounts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform         TEXT NOT NULL,               -- 'instagram' | 'vk'

    -- Instagram: instagram_business_account_id (17841405879907238)
    -- VK: group_id
    platform_id      TEXT NOT NULL,

    display_name     TEXT,
    avatar_url       TEXT,

    -- Зашифрованный AES-256-GCM токен
    -- Instagram: Page Access Token
    -- VK: Community Token
    access_token     TEXT NOT NULL,
    token_expires_at TIMESTAMPTZ,

    -- Instagram only: Facebook Page ID (555955811455114)
    page_id          TEXT,

    -- Дополнительные данные в JSON
    -- Instagram: { "page_name": "Artyom", "ig_scoped_id": "..." }
    -- VK: { "group_id": 123, "group_name": "...", "confirmation_token": "..." }
    extra            JSONB DEFAULT '{}',

    is_active        BOOLEAN NOT NULL DEFAULT true,
    status           TEXT NOT NULL DEFAULT 'running', -- 'running' | 'paused' | 'error'
    status_message   TEXT,                        -- текст ошибки если status='error'

    created_at       TIMESTAMPTZ DEFAULT now(),
    updated_at       TIMESTAMPTZ DEFAULT now(),

    UNIQUE (user_id, platform, platform_id)
);
```

### triggers

```sql
CREATE TABLE triggers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,

    -- Тип события
    event_type      TEXT NOT NULL,  -- 'comment' | 'dm' | 'comment_and_dm'

    -- Правило матчинга
    match_mode      TEXT NOT NULL DEFAULT 'keyword', -- 'keyword' | 'all' | 'regex'
    keywords        TEXT[] DEFAULT '{}',             -- ['привет', 'hello']
    keywords_mode   TEXT NOT NULL DEFAULT 'contains',-- 'contains' | 'exact' | 'starts_with'
    case_sensitive  BOOLEAN NOT NULL DEFAULT false,

    -- Действия при срабатывании на КОММЕНТАРИЙ
    reply_to_comment     BOOLEAN DEFAULT true,   -- публичный ответ в треде
    reply_comment_text   TEXT,                   -- текст публичного ответа
    send_private_reply   BOOLEAN DEFAULT false,  -- личное сообщение комментатору (Private Reply)
    private_reply_text   TEXT,                   -- текст приватного ответа (окно: 7 дней)

    -- Действия при срабатывании на ЛС
    send_dm              BOOLEAN DEFAULT false,   -- ответить в диалог (окно: 24 часа)
    dm_text              TEXT,

    -- Проверка подписки
    check_subscription        BOOLEAN NOT NULL DEFAULT false,
    reply_if_subscribed       TEXT,
    reply_if_unsubscribed     TEXT,

    -- Лимиты
    cooldown_seconds          INT DEFAULT 0,      -- пауза между ответами одному юзеру
    max_replies_per_user      INT DEFAULT 0,      -- 0 = без ограничений

    -- Приоритет матчинга (выше = проверяется раньше)
    priority        INT NOT NULL DEFAULT 0,

    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
```

### trigger_logs

```sql
CREATE TABLE trigger_logs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_id        UUID NOT NULL REFERENCES triggers(id) ON DELETE CASCADE,
    account_id        UUID NOT NULL REFERENCES connected_accounts(id),
    event_type        TEXT NOT NULL,              -- 'comment' | 'dm'
    platform_event_id TEXT,                       -- id события от платформы
    sender_id         TEXT NOT NULL,              -- platform user id отправителя
    sender_username   TEXT,
    incoming_text     TEXT,
    matched_keyword   TEXT,
    action_taken      TEXT NOT NULL,              -- 'replied_comment' | 'sent_dm' | 'both' | 'skipped'
    error_message     TEXT,
    created_at        TIMESTAMPTZ DEFAULT now()
);
```

### refresh_tokens

```sql
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       TEXT UNIQUE NOT NULL,             -- bcrypt hash
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now()
);
```

### platform_settings

Глобальный admin-«рубильник» для каждой платформы. Когда `enabled = false`: обработка
событий платформы прекращается, VK-воркеры останавливаются, а подключение новых аккаунтов
этой платформы блокируется (`403 platform_disabled`). Источник истины — БД; значение
кэшируется в Redis (`settings:platform:<platform>`, TTL 60s) и публикуется через pub/sub
`platform:state:<platform>` для мгновенной реакции VK WorkerManager. Отсутствие строки
трактуется приложением как «включено» (fail-open).

```sql
CREATE TABLE platform_settings (
    platform   TEXT PRIMARY KEY,            -- 'instagram' | 'vk'
    enabled    BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## Индексы

```sql
-- Быстрый поиск активных триггеров аккаунта
CREATE INDEX idx_triggers_account_active
    ON triggers(account_id, is_active, priority DESC);

-- Поиск по ключевым словам (GIN индекс для массива)
CREATE INDEX idx_triggers_keywords
    ON triggers USING GIN(keywords);

-- Лог срабатываний
CREATE INDEX idx_trigger_logs_account_time
    ON trigger_logs(account_id, created_at DESC);

CREATE INDEX idx_trigger_logs_trigger
    ON trigger_logs(trigger_id, created_at DESC);

-- Аккаунты пользователя
CREATE INDEX idx_accounts_user_active
    ON connected_accounts(user_id, is_active);

-- Активные подписки
CREATE INDEX idx_subscriptions_user_active
    ON subscriptions(user_id, is_active, expires_at);
```

---

## Шифрование токенов

```go
// pkg/crypto/aes.go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
)

func Encrypt(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(encoded string, key []byte) (string, error) {
    data, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil {
        return "", err
    }

    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)

    nonceSize := gcm.NonceSize()
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]

    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    return string(plaintext), err
}
```

---

## Структура миграций (goose)

```
backend/internal/db/migrations/
├── 001_create_users.sql
├── 002_create_subscriptions.sql
├── 003_create_connected_accounts.sql
├── 004_create_triggers.sql
├── 005_create_trigger_logs.sql
├── 006_create_refresh_tokens.sql
├── 007_create_indexes.sql
├── 008_make_trigger_id_nullable.sql
└── 009_create_platform_settings.sql
```

Пример миграции:

```sql
-- +goose Up
CREATE TABLE users (
    ...
);

-- +goose Down
DROP TABLE users;
```

---

## Лимиты по планам (в коде)

```go
type PlanLimits struct {
    MaxAccounts         int
    MaxTriggersPerAccount int
    LogRetentionDays    int
    Platforms           []string
}

var Plans = map[string]PlanLimits{
    "basic": {
        MaxAccounts:           2,
        MaxTriggersPerAccount: 5,
        LogRetentionDays:      7,
        Platforms:             []string{"vk"},  // или instagram, на выбор
    },
    "pro": {
        MaxAccounts:           10,
        MaxTriggersPerAccount: 50,
        LogRetentionDays:      30,
        Platforms:             []string{"vk", "instagram"},
    },
    "enterprise": {
        MaxAccounts:           -1, // без лимита
        MaxTriggersPerAccount: -1,
        LogRetentionDays:      90,
        Platforms:             []string{"vk", "instagram"},
    },
}
```
