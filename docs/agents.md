# agents.md — Instructions for AI Agents

> This file must be read by the AI agent before starting any work on the project.
> It contains all rules, conventions, and context required for correct implementation.

---

## Project Overview

**SocialSentry** — a platform for automated replies to Instagram Direct messages / comments
and VK private messages / wall comments.
Monorepo: Go backend + React frontend.

Before doing any work, read:
- [`architecture.md`](./architecture.md) — stack, diagrams, file structure
- [`database.md`](./database.md) — DB schema (tables, indexes, encryption)
- [`instagram-api.md`](./instagram-api.md) — how the Instagram API works (verified in practice)
- [`vk-api.md`](./vk-api.md) — how the VK API works
- [`plans.md`](./plans.md) — subscription plans/limits + where users view logs

---

## Critical Context: Instagram API

> This is the most important section. Do not make assumptions — read carefully.

### One Domain, One Token — for Everything

```
✅ CORRECT:   graph.facebook.com + Page Access Token (EAAZAjwU...)
❌ INCORRECT: graph.instagram.com + Instagram User Token (IGAA...)
❌ INCORRECT: mixing two different token types
```

All Instagram functionality — sending DMs, reading conversations, replying to comments —
is implemented via `graph.facebook.com` using a **Page Access Token**.
No IGAA Instagram tokens are used anywhere in this project.

### Verified Endpoints

```
# Reply to a DM ✅ VERIFIED
POST https://graph.facebook.com/v21.0/{page_id}/messages
Body: {
  "recipient":      {"id": "{sender_ig_scoped_id}"},
  "message":        {"text": "reply text"},
  "messaging_type": "RESPONSE",
  "access_token":   "{page_access_token}"
}

# Public reply in a comment thread ✅ VERIFIED
POST https://graph.facebook.com/v21.0/{comment_id}/replies
Body: {
  "message":      "reply text",
  "access_token": "{page_access_token}"
}

# Private Reply — send a DM triggered by a comment ✅ SUPPORTED
# Same endpoint as DM, but recipient uses comment_id instead of id
POST https://graph.facebook.com/v21.0/{page_id}/messages
Body: {
  "recipient":      {"comment_id": "{comment_id}"},
  "message":        {"text": "Sent you details in DM!"},
  "messaging_type": "RESPONSE",
  "access_token":   "{page_access_token}"
}

# List conversations ✅ VERIFIED
GET https://graph.facebook.com/v21.0/{page_id}/conversations
  ?platform=instagram&limit=5&fields=id,participants&access_token={page_access_token}
```

### Comment Trigger — Three Possible Actions

When a comment event fires, a trigger can execute any combination of:

```
1. reply_to_comment = true   → POST /{comment_id}/replies
                               Visible public reply in the comment thread

2. send_dm = true            → POST /{page_id}/messages
                               with recipient: {"comment_id": "{comment_id}"}
                               Private DM to the commenter (window: 7 days)

3. Both simultaneously       → Execute action 1 then action 2
                               Most powerful combo for engagement
```

Window for private reply from comment: **7 days** (vs 24h for regular DM replies).
Only one private message can be sent this way per comment.
Full dialog opens only if the user replies back to the DM.

### Key Variable Values (from live testing)

```
page_id             = "555955811455114"    // Facebook Page ID
ig_user_id          = "17841405879907238"  // Instagram Business Account ID
app_scoped_id       = "26851625147828724"  // App-scoped user ID — DO NOT use in API calls
test_sender_id      = "1644146630022851"   // zudina.anyaa (verified test recipient)
app_id              = "1388425723092623"
```

### Hard Constraints (enforce strictly in code)

- Can only **reply** to incoming messages. Initiating a conversation is not allowed.
- DM reply window: **24 hours** after the user's DM (`messaging_type: RESPONSE`).
- Private reply from comment window: **7 days** from the comment — one message only.
- Rate limit: **200 API calls / account / hour** — implement token bucket in Redis.
- In Development Mode, API only sees conversations with users who have a role in the Meta App.
- Before sending any message, the app **must** be subscribed to the page:
  `POST /{page_id}/subscribed_apps?subscribed_fields=messages,...` — do this once on account connect.

### Recipient Field Differs by Action Type

```go
// Reply to a DM
"recipient": {"id": senderIgScopedID}

// Private reply from a comment (DM triggered by comment)
"recipient": {"comment_id": commentID}

// Public comment reply — different endpoint entirely
POST /{comment_id}/replies  (no recipient field needed)
```

### Token Refresh

Page Access Token lives ~60 days. Asynq cron task `token_refresh` renews it on day 50:
```
GET https://graph.facebook.com/v21.0/oauth/access_token
  ?grant_type=fb_exchange_token
  &fb_exchange_token={current_page_access_token}
  &client_id={META_APP_ID}
  &client_secret={META_APP_SECRET}
```

---

## Critical Context: VK API

### How the Worker Operates

VK uses **Bots Long Poll API** — the worker pulls events itself (pull, not push).
One goroutine per account, blocking `lp.Run()`, exits via `context.Cancel()`.

### SDK Import

```go
import (
    "github.com/SevereCloud/vksdk/v3/api"
    "github.com/SevereCloud/vksdk/v3/events"
    "github.com/SevereCloud/vksdk/v3/longpoll-bot"
)
```

### Subscription Check

```go
// Cache result in Redis TTL=5min to avoid hitting the API on every message
result, err := vk.GroupsIsMember(api.Params{
    "group_id": groupID,
    "user_id":  userID,
})
isMember := result == 1
```

### Rate Limits

- 20 API requests / second per token → implement token bucket rate limiter
- `random_id` in `messages.send` must be unique on every call

---

## Go Code Conventions

### Package Responsibilities

Follow the structure from [`architecture.md`](./architecture.md). Summary:

```
internal/handler/    — HTTP only (parse request → call service → write response)
internal/service/    — business logic (no HTTP, no direct DB access)
internal/repository/ — SQL only (interface + pgx implementation)
internal/platform/   — external API clients (Instagram, VK)
internal/engine/     — Bot Engine (WorkerManager, TriggerMatcher)
internal/queue/      — Asynq task definitions and handlers
pkg/                 — utilities with no dependencies on internal packages
```

### Mandatory Rules

```go
// ✅ Always wrap errors with context
if err != nil {
    return fmt.Errorf("service.CreateTrigger: %w", err)
}

// ✅ Always pass context as the first argument
func (r *TriggerRepo) GetByAccount(ctx context.Context, accountID string) ([]Trigger, error)

// ✅ Platform tokens must always go through crypto.Encrypt/Decrypt
token, err := crypto.Encrypt(rawToken, cfg.EncryptionKey)

// ✅ Use zap for logging — never fmt.Println
logger.Info("worker started", zap.String("account_id", accountID))

// ✅ Always terminate goroutines via context
ctx, cancel := context.WithCancel(parent)
defer cancel()

// ❌ Never store tokens as plain text in the DB
account.AccessToken = rawToken // FORBIDDEN
```

### Naming Conventions

```go
// Structs — PascalCase
type TriggerMatcher struct {}

// Methods — camelCase
func (m *TriggerMatcher) findMatch(text string) (*Trigger, error)

// Platform constants
const (
    PlatformInstagram = "instagram"
    PlatformVK        = "vk"
)

// Event type constants
const (
    EventTypeDM           = "dm"
    EventTypeComment      = "comment"
    EventTypeCommentAndDM = "comment_and_dm"
)

// Match mode constants
const (
    MatchModeKeyword = "keyword"
    MatchModeAll     = "all"
    MatchModeRegex   = "regex"
)
```

### HTTP Response Standard

```go
// Success — single item
c.JSON(http.StatusOK, gin.H{"data": result})

// Success — list with pagination
c.JSON(http.StatusOK, gin.H{
    "data": items,
    "meta": gin.H{"page": page, "per_page": perPage, "total": total},
})

// Created
c.JSON(http.StatusCreated, gin.H{"data": result})

// Validation error
c.JSON(http.StatusBadRequest, gin.H{
    "error":   "validation_error",
    "message": "Keywords cannot be empty",
    "details": details,
})

// No active subscription
c.JSON(http.StatusForbidden, gin.H{
    "error":               "subscription_required",
    "message":             "An active subscription is required",
    "subscription_status": "expired",
})

// Not found
c.JSON(http.StatusNotFound, gin.H{
    "error":   "not_found",
    "message": "Trigger not found",
})

// Internal error
c.JSON(http.StatusInternalServerError, gin.H{
    "error":   "internal_error",
    "message": "Internal server error",
})
```

---

## React / TypeScript Conventions

### Component Structure

```typescript
// ✅ Named exports preferred
export function TriggerEditor({ accountId }: TriggerEditorProps) {}

// ✅ Props interface declared next to the component
interface TriggerEditorProps {
    accountId: string
    onSave?: () => void
}

// ✅ TanStack Query for all server state
const { data, isLoading, error } = useQuery({
    queryKey: ['triggers', accountId],
    queryFn: () => api.triggers.list(accountId),
})

// ✅ Zustand only for global client state (auth token, websocket connection)
// Server state belongs in TanStack Query, not Zustand

// ✅ React Hook Form + zod for all forms
const schema = z.object({
    name:     z.string().min(1, 'Required'),
    keywords: z.array(z.string()).min(1),
})
```

### Axios Interceptors (pre-configured in api/client.ts)

```typescript
// Automatically attaches Bearer token to every request
// On 401 — refreshes the access token and retries the original request
// On 403 subscription_required — redirects to /subscription page
```

### Route Guards

```typescript
// router.tsx
// <ProtectedRoute>     — validates JWT, redirects to /login if missing
// <SubscriptionRoute>  — validates active subscription, shows banner if expired
// <AdminRoute>         — validates role === 'admin', redirects to /dashboard if not
```

---

## Subscription System Rules

### What the Middleware Checks (backend)

```
RequireActiveSubscription verifies:
1. A subscription record exists for the user_id
2. is_active = true
3. expires_at IS NULL (lifetime) OR expires_at > now()

On failure → 403 subscription_required
On success → sets subscription object in gin context
```

### Blocked Without an Active Subscription

- `POST /api/v1/accounts/*` — connecting new accounts
- `POST/PUT/DELETE /api/v1/accounts/:id/triggers/*` — creating or modifying triggers
- `GET /api/v1/accounts/:id/triggers/:tid/logs` — viewing trigger logs
- Worker startup — WorkerManager checks subscription before launching any worker

### NOT Blocked Without a Subscription

- `GET /api/v1/accounts` — viewing already connected accounts
- `GET /api/v1/subscription` — viewing own subscription status
- `GET /api/v1/accounts/:id/triggers` — viewing triggers (editing is blocked)

### Subscriptions Are Granted by Admin Only

There is no public self-service endpoint for subscriptions.
`POST /api/v1/admin/subscriptions` is restricted to users with role `admin`.

---

## Bot Engine Implementation Rules

### WorkerManager Startup

```go
// On application start, launch workers ONLY for accounts where:
// 1. connected_accounts.is_active = true
// 2. connected_accounts.status != 'error'
// 3. The account owner has an active subscription

// Redis pub/sub channels:
// "worker:start:{account_id}"    → start worker for that account
// "worker:stop:{account_id}"     → stop worker for that account
// "triggers:reload:{account_id}" → invalidate trigger cache, reload on next match
```

### TriggerMatcher — Check Order

```
1. is_active = true                      → skip if false
2. event_type matches                    → 'dm' | 'comment' | 'comment_and_dm'
3. match_mode evaluation                 → keyword / all / regex
4. cooldown check (Redis)                → skip if sender is in cooldown window
5. max_replies_per_user (Redis counter)  → skip if per-user limit exceeded
6. check_subscription (if enabled)       → determine which reply text to use
7. First trigger matched by priority DESC wins — remaining triggers not evaluated
```

### Trigger Cache

```go
// Redis key: "triggers:{account_id}"
// TTL: 60 seconds
// On trigger create/update/delete via API → immediately DEL the key
// On next TriggerMatcher call → reload from DB and re-cache
```

### Account Status Values

```
"running" — worker is active and processing events
"paused"  — manually paused by the user via UI
"error"   — worker crashed (e.g. invalid/expired token)
            → write error details to status_message
            → send WebSocket notification to the account owner
            → do not auto-restart (user must reconnect)
```

---

## Database Rules

### Never Do

```sql
-- ❌ Never SELECT * — always name the columns you need
SELECT * FROM triggers WHERE account_id = $1;

-- ❌ Never store raw tokens in the DB
INSERT INTO connected_accounts (access_token) VALUES ('EAAZAjwU_raw');

-- ❌ Never delete logs directly — only via scheduled Asynq maintenance task
DELETE FROM trigger_logs WHERE created_at < now() - interval '7 days';
```

### Always Do

```sql
-- ✅ Always isolate by user_id — never trust account_id from request alone
SELECT t.id, t.name, t.keywords FROM triggers t
JOIN connected_accounts a ON t.account_id = a.id
WHERE a.user_id = $1 AND t.id = $2;

-- ✅ Always paginate lists
SELECT id, sender_id, action_taken, created_at
FROM trigger_logs
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- ✅ Always update updated_at on any mutation
UPDATE triggers SET name = $1, updated_at = now() WHERE id = $2;
```

---

## Reply Template Variables

Implement in `engine/matcher.go`:

```go
func ApplyTemplate(text string, data TemplateData) string {
    replacer := strings.NewReplacer(
        "{{name}}",     data.SenderName,
        "{{username}}", data.SenderUsername,
        "{{keyword}}",  data.MatchedKeyword,
        "{{time}}",     data.EventTime.Format("15:04"),
        "{{date}}",     data.EventTime.Format("02.01.2006"),
    )
    return replacer.Replace(text)
}

type TemplateData struct {
    SenderName     string
    SenderUsername string
    MatchedKeyword string
    EventTime      time.Time
}
```

---

## Environment Variables — Source Reference

```go
// config/config.go
type Config struct {
    DB struct {
        URL string        // DATABASE_URL
    }
    Redis struct {
        URL string        // REDIS_URL
    }
    JWT struct {
        Secret     string        // JWT_SECRET
        AccessTTL  time.Duration // JWT_ACCESS_TTL  (default: 15m)
        RefreshTTL time.Duration // JWT_REFRESH_TTL (default: 168h)
    }
    Encryption struct {
        Key []byte        // ENCRYPTION_KEY (hex string decoded to bytes)
    }
    Meta struct {
        AppID              string // META_APP_ID
        AppSecret          string // META_APP_SECRET
        WebhookVerifyToken string // META_WEBHOOK_VERIFY_TOKEN
        CallbackURL        string // META_CALLBACK_URL
    }
    VK struct {
        APIVersion string // VK_API_VERSION (default: 5.199)
    }
    Server struct {
        Port        string // PORT (default: 8080)
        Environment string // ENVIRONMENT: development | production
    }
}
```

---

## Platform Error Handling

### Instagram API Errors

```go
// On any send failure:
// 1. Write to trigger_logs: action_taken="error", error_message=err.Error()
// 2. Error code 190 (invalid/expired token) → set account status="error",
//    write status_message, send WebSocket notification to user
// 3. Error code 10 + subcode 2534022 (outside 24h window) →
//    log as skipped, do NOT mark as error, do NOT retry
// 4. Error code 613 (rate limit) → re-enqueue in Asynq with delay=5min
// 5. All other errors → log with zap.Error, retry via Asynq (max 3 attempts)
```

### VK API Errors

```go
// Error code 5  (invalid token)  → status="error", WebSocket notification
// Error code 9  (flood control)  → exponential backoff, retry
// Error code 15 (no access)      → status="error", WebSocket notification
// All other errors               → log with zap.Error, retry up to 3 times with backoff
```

---

## Testing

### Unit Tests — Required Coverage

```
engine/matcher.go           — all match modes, edge cases (empty text, regex errors, cooldown)
middleware/subscription.go  — expired, missing, and active subscription scenarios
pkg/crypto/aes.go           — encrypt/decrypt roundtrip, wrong key returns error
```

### Integration Tests

```
handler/auth_test.go      — register, login, refresh, logout full flow
handler/webhooks_test.go  — Meta signature verification, event parsing, 200 immediate response
```

### Test Infrastructure

```go
// Use testcontainers-go for PostgreSQL and Redis in integration tests
// Do not mock the database — test against a real instance
// Seed test data via goose migrations + SQL fixtures
```

---

## Anti-Patterns — Never Do These

```
❌ Do not make synchronous calls to Instagram/VK API inside a webhook handler
   → Always enqueue to Asynq and respond 200 immediately

❌ Do not load all triggers for all accounts on startup
   → Load lazily per account on first event, cache in Redis

❌ Do not persist worker state in PostgreSQL
   → Worker state lives in WorkerManager memory + Redis pub/sub only

❌ Do not send multiple replies to the same event
   → Platforms may flag or ban the account for spam

❌ Do not log tokens, secrets, or access tokens
   → Use zap field masking; never log raw credential values

❌ Do not query the DB directly inside TriggerMatcher
   → Always go through Redis cache; hit DB only on cache miss

❌ Do not allow a user to access another user's accounts or triggers
   → Every request must verify user_id from JWT against the resource's owner

❌ Do not use graph.instagram.com or IGAA tokens
   → All Instagram operations use graph.facebook.com + Page Access Token
```
