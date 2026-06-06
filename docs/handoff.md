# SocialSentry — Agent Handoff (post-Phase 4)

This document is the source of truth for an incoming agent on the SocialSentry repo. **Phases 1, 2, 3, and 4 are complete.** The backend builds/vets/tests clean and the frontend builds clean. Phase 5 (Polish) is the remaining work.

> Canonical location: this file (`docs/handoff.md`) inside the repo. A mirror may exist in a local Claude plans cache, but the repo copy is authoritative.

## TL;DR

- **Backend**: `go -C backend build ./...` → exit 0. `go -C backend vet ./...` → exit 0. `go -C backend test ./...` → green, **48 unit tests pass** (was 41; +7 for `engine.TestTrigger`). Also green under `-race` — see § Enabling -race.
- **Frontend**: `cd frontend && npm run build` → 196 modules, ~150 kB gzipped, 0 TS errors.
- **Phase 4 (VK)**: fully wired — VK client, Bots Long Poll workers, per-account WorkerManager with Redis pub/sub, connect handler, subscription checker, frontend connect form. See § Phase 4.
- **Gap #1 (skipped logs) — CLOSED**: migration `008` made `trigger_logs.trigger_id` NULLABLE; both the IG handler and the VK dispatcher now persist `action_taken='skipped'` rows.
- **Gap #2 (trigger cache hot-reload) — CLOSED**: `queue/pubsub.go` Publisher + service-side `publishReload`; the VK WorkerManager subscribes to `triggers:reload:*`.
- **Gap #A (VK subscription-gating token bug) — CLOSED**: the dispatcher decrypts the community token before the matcher's `ChooseReplyText`, so `groups.isMember` authenticates. Covered by new VK unit tests.
- **Phase 5 #5 (Test trigger endpoint) — SHIPPED previously**: `POST /api/v1/accounts/:id/triggers/:tid/test` (auth-only, no sub gate). See § Phase 5 #5 shipped.
- **Phase 5 batch #4 / #6 / #7 / #8 / #9 / #12 — SHIPPED this session**. Skipped-log filter, plan-limit dialog, DnD priority reordering, IG token-refresh cron, log-retention cron, golangci-lint + GitHub Actions CI. See § Phase 5 session 2 shipped.
- **Project**: SocialSentry — Instagram + VK auto-reply SaaS. Go backend + React frontend + PostgreSQL + Redis + Asynq (IG) + Bots Long Poll (VK).
- **Phase order** (user-approved): Foundation → Subscriptions/Admin → Instagram → **VK** → Polish.

---

## Repository layout

```
F:\Git\SocialSentry\
├── CLAUDE.md                       # AI agent conventions (mirror of docs/agents.md)
├── README.md                       # thin landing page → docs/README.md
├── docker-compose.yml              # postgres 16 + redis 7 + api + worker + frontend
├── .env / .env.example             # secrets (JWT_SECRET, ENCRYPTION_KEY are 64-char hex) + VK_API_VERSION
├── docs/
│   ├── plan.md                     # original product plan
│   ├── handoff.md                  # THIS FILE — current agent handoff
│   ├── agents.md / architecture.md / database.md / deployment.md
│   ├── instagram-api.md / vk-api.md / README.md
├── backend/
│   ├── cmd/
│   │   ├── api/main.go             # Gin HTTP server, full composition root (IG + VK routes)
│   │   ├── worker/main.go          # Asynq server (IG) + VK WorkerManager, both started together
│   │   └── encrypt-token/main.go   # ONE-OFF dev helper: encrypts a string with ENCRYPTION_KEY
│   ├── internal/
│   │   ├── config/config.go        # viper loader; Meta + VK sections; validates JWT/ENCRYPTION_KEY
│   │   ├── db/
│   │   │   ├── migrations/         # 8 goose migrations (008 = trigger_id NULLABLE)
│   │   │   ├── query/              # users, refresh_tokens, accounts, triggers, trigger_logs, subscriptions, users_admin
│   │   │   └── generated/          # sqlc output (DO NOT HAND-EDIT)
│   │   ├── domain/                 # struct files + constants (PlatformInstagram, PlatformVK, etc.)
│   │   ├── repository/             # interfaces + pgx impls; uuid helpers in pgtype.go; errors.go
│   │   ├── service/                # auth, account, trigger, subscription, admin + plans.go (limits)
│   │   ├── handler/                # auth, accounts, accounts_instagram, accounts_vk, triggers, webhooks, admin
│   │   ├── middleware/             # auth, subscription, logger, ratelimit
│   │   ├── engine/                 # matcher.go, template.go (platform-neutral); SubscriptionChecker iface
│   │   ├── platform/instagram/     # client + oauth + messages + comments + webhook + connect
│   │   ├── platform/vk/            # client, messages, comments, subscription, dispatcher, worker, manager
│   │   └── queue/                  # client.go (Asynq producer) + pubsub.go (Publisher) + handlers/instagram.go
│   ├── pkg/
│   │   ├── crypto/aes.go           # AES-256-GCM for platform tokens
│   │   └── jwt/jwt.go              # HS256 with Claims{UserID, Role, RegisteredClaims}
│   ├── go.mod                      # module .../backend, go 1.25.0; includes SevereCloud/vksdk/v3 v3.3.1
│   ├── sqlc.yaml / .air.toml / Dockerfile
└── frontend/
    ├── src/
    │   ├── api/                    # axios client + TanStack Query hooks (auth, accounts incl. useConnectVK, triggers, subscription, admin)
    │   ├── components/             # ui/, layout/DashboardLayout, SubscriptionBanner
    │   ├── pages/                  # auth, dashboard, accounts (IG + VK connect), triggers, logs, subscription, admin
    │   ├── store/auth.ts           # Zustand, persisted
    │   ├── router.tsx              # Protected + AdminRoute guards
    │   └── main.tsx                # QueryClient + RouterProvider + Toaster
    ├── components.json / package.json
```

---

## Local environment

### Prereqs
- Go 1.25+ (1.22+ required by `go.mod`)
- Node 18+ (some shadcn CLI features need 20+ — see § Known gaps)
- Docker Desktop running
- `curl`, `openssl` for HMAC smoke (IG); VK needs no HMAC

### Standard run
```powershell
# 1. Infra
docker compose up -d postgres redis

# 2. Migrations (now 8 total)
$env:DATABASE_URL = 'postgres://socialsentry:secret@localhost:5432/socialsentry?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@latest -dir backend/internal/db/migrations postgres $env:DATABASE_URL up

# 3. Load .env into process env (DATABASE_URL, REDIS_URL, JWT_SECRET, ENCRYPTION_KEY, META_*, VK_API_VERSION, PORT, ENVIRONMENT, LOG_LEVEL)

# 4. API + worker (worker now runs Asynq for IG AND the VK long-poll manager)
go -C backend run ./cmd/api
go -C backend run ./cmd/worker

# 5. Frontend
cd frontend; npm run dev
```

### Env vars added in Phase 4
- `VK_API_VERSION` — defaults to `5.199` in `config/config.go` if unset.

### Seeded test users (from Phase 3 smoke; may still be in dev DB)
| Email | Password | Role | UUID |
|---|---|---|---|
| `e2e-1@test.local` | `longpass123` | **admin** | `75fcec57-603e-412e-9583-f62246cf0e78` |
| `e2e-2@test.local` | `longpass456` | user | `10b46b4a-2804-4bfd-9c2a-0380c08d693a` |

`e2e-2` has an active Pro subscription and a fake IG account (id `966f1c76-0dd3-4990-a77a-cb9abca8e00c`) used for the IG smoke. The fake Page token causes Meta `code 190` on send — expected.

---

## Phase 1 — Foundation (DONE)

- Monorepo skeleton (Gin api, zap worker, full `internal/`+`pkg/` tree, docker-compose, .env).
- DB schema: goose migrations (pgcrypto + users, subscriptions, connected_accounts, triggers, trigger_logs, refresh_tokens, indexes). sqlc for users + refresh_tokens.
- `config/config.go` viper loader (validates JWT_SECRET + 32-byte ENCRYPTION_KEY).
- `pkg/crypto/aes.go` AES-256-GCM (6 tests). `pkg/jwt/jwt.go` HS256 (7 tests).
- Auth API: bcrypt(12); refresh tokens = 32 random bytes, stored as SHA-256 hash; httpOnly SameSite=Strict cookie; rate-limited login/register. Integration test (testcontainers).
- React auth + dashboard shell: axios refresh interceptor, Zustand store, route guards, login/register forms.

## Phase 2 — Subscriptions + Admin (DONE)

- `service/subscription.go` (Grant/Update/Revoke/GetActive/List — only one active sub per user).
- `service/admin.go` (ListUsers/SetRole/SetBlocked/GetStats — UUIDs threaded properly).
- `service/plans.go` `PlanLimitsByName`:

| Plan | MaxAccounts | MaxTriggers | LogDays | Platforms | MultiplePlatforms |
|---|---|---|---|---|---|
| basic | 2 | 5 | 7 | VK + IG | **false** (one platform at a time) |
| pro | 10 | 50 | 30 | VK + IG | true |
| enterprise | -1 (∞) | -1 (∞) | 90 | VK + IG | true |

- `middleware/subscription.go` `RequireActiveSubscription` + `RequireAdmin`.
- Frontend: admin users/subscriptions pages, subscription status view, SubscriptionBanner.

## Phase 3 — Instagram + Engine Core (DONE)

- Accounts + Triggers + Logs CRUD (sqlc queries, pgx repos, services with validation sentinels, handlers with error mapping).
- `engine/template.go` `ApplyTemplate` ({{name}}, {{username}}, {{keyword}}, {{time}}, {{date}}).
- `engine/matcher.go` `TriggerMatcher`: 60s in-process cache, priority DESC, event-type + text match, Redis cooldown + max_replies, `RecordFire`, `ChooseReplyText` (subscription gating), `InvalidateCache`.
- `platform/instagram/`: client (typed APIError + rate limit), messages (SendDM/SendPrivateReply), comments (ReplyToComment), oauth (full 6-step), webhook (HMAC verify + parse), connect (CompleteOAuth).
- `handler/webhooks.go` (Verify + Receive→enqueue) and `handler/accounts_instagram.go` (Connect + Callback with Redis state).
- `queue/handlers/instagram.go` `InstagramHandler` full pipeline; `handleAPIError` flips account to `error` on code 190.
- Frontend: accounts list with IG connect, trigger list + full editor, paginated logs.

---

## Phase 4 — VK adapter (DONE)

VK is a Bots-Long-Poll adapter onto the same platform-neutral engine/matcher. Unlike IG (webhooks → Asynq), VK uses a long-lived goroutine per active community.

### `internal/platform/vk/`
- **`client.go`** — `Client{VK *vksdk.VK, GroupID, AccountID, rdb, rateLimit}`. `NewClient(token, groupID, accountID, apiVersion, rdb)`. `DefaultRateLimit=18` rps. `CheckRateLimit` = Redis INCR + EXPIRE 1s on `ratelimit:vk:<accountID>` → `ErrRateLimited`. Error classifiers via `*vksdk.Error`: `IsAuthError` (code 5), `IsFloodControl` (code 9), `IsNoAccess` (code 15). `VerifyToken` → `groups.getById` returns `GroupInfo{ID, Name}`.
- **`messages.go`** — `SendMessage(ctx, userID, text)` → `messages.send` with `random_id = time.Now().UnixNano()`.
- **`comments.go`** — `ReplyToWallComment(ctx, ownerID, postID, replyToCommentID, text)` → `wall.createComment` with `from_group=1`. For community-owned walls `ownerID = -groupID`.
- **`subscription.go`** — `SubscriptionChecker{rdb, apiVersion}` implements `engine.SubscriptionChecker`. `IsSubscribed` returns `(false, nil)` for non-VK accounts; else Redis-cached (`vk_member:<group>:<user>`, TTL 5m) `groups.isMember`. The VK dispatcher pre-decrypts the token (see `Dispatcher.decryptedAccount`) before the matcher reaches here.
- **`dispatcher.go`** — `Dispatcher` orchestrates one event: `lookupActive` → `matcher.Match` → (skip→`recordSkipped`) → `decryptedAccount` (decrypts the token once onto an account copy) → `clientFor` → `ApplyTemplate` → `ChooseReplyText` (now sees the decrypted token) → `SendMessage`/`ReplyToWallComment` → `recordOK`/`recordError` + `RecordFire`. `handleAPIError` flips account to `error` on code 5/15. `recordSkipped` now persists a NULL-trigger log (Gap #1 fix).
- **`worker.go`** — `AccountWorker` runs `longpoll-bot` for one community; decrypts token, builds client, wires `MessageNew`+`WallReplyNew` callbacks to the dispatcher, `RunWithContext` until cancel/fatal. On fatal error → `handleAPIError`. Plus a `sync.Mutex`-guarded `workerRegistry` keyed by account_id.
- **`manager.go`** — `WorkerManager.StartAll(ctx)` spawns a worker per active VK account (`accountRepo.ListAllActive` filtered to `PlatformVK`), then `runPubSub` subscribes to `worker:start:*`, `worker:stop:*`, `triggers:reload:*`. `spawn`/`stop` manage goroutine lifecycle; `Shutdown` cancels all. Channel name constants live here AND in `queue/pubsub.go` (kept identical).

### Connect handler + routing
- **`handler/accounts_vk.go`** — `POST /api/v1/accounts/vk/connect` (auth + active-sub gated). Body `{group_id, community_token}`. Validates `group_id` is a positive int, calls `VerifyToken` (groups.getById) to confirm the token↔group pair, persists via `accountSvc.CreateConnected` (`platform_id=group_id`, `extra={group_id, group_name}`, token encrypted), then publishes `worker:start:<account_id>`. Maps `ErrConflict`/`ErrAccountLimitExceeded`/`ErrAccountPlatformNotAllowed` to friendly 4xx.
- **`cmd/api/main.go`** — route wired under the `gated` group: `gated.POST("/accounts/vk/connect", vkConnectH.Connect)`.
- **`cmd/worker/main.go`** — builds `vk.NewSubscriptionChecker` → injected into `engine.NewTriggerMatcher`; builds `vk.NewDispatcher` + `vk.NewWorkerManager`; starts `vkManager.StartAll(ctx)` in a goroutine next to the Asynq server; `vkManager.Shutdown()` on SIGINT/SIGTERM.
- **`config/config.go`** — `VKConfig{APIVersion}`, default `5.199`.
- **`go.mod`** — `github.com/SevereCloud/vksdk/v3 v3.3.1`.

### Frontend
- **`src/api/accounts.ts`** — `useConnectVK()` posts to `/accounts/vk/connect`.
- **`src/pages/accounts/AccountsList.tsx`** — "Подключить VK" toggles a zod-validated form (`group_id` digits-only, `community_token` min length); success/error toasts; platform label + `group_id=` display for VK rows. IG callback query-string handling unchanged (VK has no callback).
- Trigger editor already exposes the subscription-check block, which is meaningful for VK.

### Decisions baked in during Phase 4 (answers to the old open questions)
- **Connect UX**: paste form (`community_token` + `group_id`), no OAuth deep link.
- **Plan limits**: option (a) — `MultiplePlatforms=false` blocks adding VK alongside IG on basic; surfaced as `platform_not_allowed`.
- **Worker restart**: no auto-restart. A fatal long-poll error flips the account to `status='error'`; user must reconnect (which republishes `worker:start`).
- **WorkerManager location**: lives in `platform/vk/manager.go` (not `engine/`), to avoid an engine→platform import.

---

## Gap closures shipped in this handoff

### Gap #1 — Skipped/ingress logs (CLOSED)
- **Migration `008_make_trigger_id_nullable.sql`**: `ALTER TABLE trigger_logs ALTER COLUMN trigger_id DROP NOT NULL` (down re-adds NOT NULL after deleting NULL rows). The `ON DELETE CASCADE` FK still applies to non-NULL rows. Verified live: `information_schema` reports `trigger_id` `is_nullable = YES`.
- **`repository/log.go`**: `Create` treats an empty `CreateLogParams.TriggerID` as a deliberate SQL `NULL` (`pgtype.UUID{Valid:false}`) instead of erroring.
- **`queue/handlers/instagram.go`**: `recordLog` no longer early-returns when `triggerID==""`; skipped DMs/comments now persist `action_taken='skipped'` with the reason in `error_message`.
- **`platform/vk/dispatcher.go`**: `recordSkipped` (previously a no-op) now writes a NULL-trigger `skipped` row with the reason in `error_message`.
- **sqlc**: no regeneration needed — sqlc already maps `trigger_id` to `pgtype.UUID` for both NOT NULL and NULL columns, so the generated types are unchanged.

Skip reasons emitted by the matcher: `cooldown`, `max_replies_reached`, `no_action_text`.

### Gap #2 — Trigger cache hot-reload (CLOSED)
- **`queue/pubsub.go`**: `Publisher` with `PublishTriggersReload`, `PublishWorkerStart`, `PublishWorkerStop` (best-effort, logs on failure). Channel constants match the VK manager.
- **`service/trigger.go`**: `publishReload(accountID)` is called after Create/Update/Delete/Toggle (via injected `pub`).
- **`service/account.go`**: takes a `WorkerLifecyclePublisher` (nil-safe) to publish `worker:start`/`worker:stop` on connect/pause/delete.
- **`platform/vk/manager.go`**: subscribes to `triggers:reload:*` and calls `matcher.InvalidateCache(accountID)`.

### Gap #A — VK subscription-gating token bug (CLOSED)
- **Root cause**: `ChooseReplyText` passed the matcher an `account` whose `AccessToken` was still encrypted, so `vk.SubscriptionChecker.IsSubscribed` built its vksdk client from ciphertext and `groups.isMember` always failed (silently falling back to default text).
- **Fix (option a)**: `platform/vk/dispatcher.go` adds `decryptedAccount(account)` → returns a copy with the plaintext token; both `HandleMessageNew` and `HandleWallReplyNew` decrypt once after a match and pass that copy to `clientFor` and `ChooseReplyText`. `clientFor` now takes the plaintext token (no double-decrypt). Engine stays decrypt-agnostic.
- **Tests**: `dispatcher_test.go` (`decryptedAccount` carries plaintext, propagates decrypt errors, doesn't mutate the input) + `subscription_test.go` (non-VK short-circuit, bad group_id/sender_id errors).

### Gap #C — VK dead code / smells (CLOSED)
- Removed `formatGroupForLog` (+ the now-unused `fmt` import) from `platform/vk/worker.go`.
- Removed the never-assigned `rdb vkRedis` field + the `vkRedis` interface type from `platform/vk/dispatcher.go`.
- Removed the `context` import + `var _ context.Context` suppression from `handler/accounts_vk.go`.
- `go build` / `go vet` / `gofmt` all clean. Remaining nit: VK connect still publishes via a raw `rdb.Publish` rather than `queue.Publisher` — left as-is to avoid widening handler deps.

---

## Tests inventory

| Package | Test file | Count | Notes |
|---|---|---|---|
| `pkg/crypto` | aes_test.go | 6 | roundtrip, wrong key, tampered, too-short, invalid base64 |
| `pkg/jwt` | jwt_test.go | 7 | roundtrip, wrong secret, expired, malformed, empty secret, admin role |
| `internal/engine` | matcher_test.go | 11 | match modes, case-sensitive, regex, priority, no-match, template |
| `internal/engine` | test_trigger_test.go | 7 | DM hit, text miss, event-type mismatch, comment dual-action, empty action text, regex hit, `comment_and_dm` accepts either kind |
| `internal/platform/instagram` | webhook_test.go | 6 | signature × 4 + parse DM + parse comment |
| `internal/handler` | auth_test.go | 1 | env-gated integration (requires TEST_DATABASE_URL + TEST_REDIS_URL; runs register→login→/me→refresh→logout→revoked 401; green under `-race` against docker Postgres/Redis) |
| `internal/handler` | webhooks_test.go | 4 | verify happy / wrong token / sig mismatch / valid 200 |
| `internal/platform/vk` | client_test.go, dispatcher_test.go, subscription_test.go | 6 | error classifiers (5/9/15 + wrapped); rate-limit budget + TTL refill (miniredis); `decryptedAccount`; `IsSubscribed` non-VK/bad-ids + Redis cache hit |
| **TOTAL** | | **48** | `go -C backend test ./... -count=1` exit 0 |

⚠️ **VK coverage is improving but not complete.** Covered: error classifiers, rate-limit (budget + TTL refill via `miniredis`), `decryptedAccount`, and `IsSubscribed` (non-VK, bad ids, Redis cache hit). Still untested: the send paths (`SendMessage`/`ReplyToWallComment`), the long-poll loop in `worker.go`, and the `WorkerManager` pub/sub lifecycle — they need a fake VK API + fake repos.

---

## Enabling `-race` (Windows dev env)

**Status: DONE on this machine.** MSYS2 + `mingw-w64-ucrt-x86_64-gcc` 16.1.0 are installed and `C:\msys64\ucrt64\bin` is on the persistent user PATH, so `go test -race` works in new shells (Go auto-enables cgo when gcc is on PATH). Verified: `go -C backend test ./... -race -count=1` → all packages green, no data races. (Existing IDE terminals may need a restart to inherit the new PATH; until then prepend it for the session.)

Original setup steps (for a fresh box):

1. **Install a C compiler (gcc).** On Windows the simplest is MSYS2 → `pacman -S mingw-w64-ucrt-x86_64-gcc`, then add `C:\msys64\ucrt64\bin` to `PATH`. (Alternatives: TDM-GCC, or `choco install mingw`.)
2. **Enable cgo**: `setx CGO_ENABLED 1` for new shells, or per-run `$env:CGO_ENABLED=1`.
3. **Verify**: `gcc --version` resolves and `go env CGO_ENABLED` → `1`.
4. **Run**: `$env:CGO_ENABLED=1; go -C backend test ./... -race -count=1`.

Notes: `-race` supports amd64/arm64 only and slows tests ~2–10×. Once gcc is on PATH, `CGO_ENABLED` is unnecessary (Go defaults it to 1 when a C compiler is found). On CI (Linux) gcc is preinstalled, so `CGO_ENABLED=1 go test -race ./...` works out of the box — that's the recommended home for `-race`.

---

## Known gaps (remaining)

### A. VK subscription gating token bug — FIXED
Resolved in this handoff via `Dispatcher.decryptedAccount` (option a). See § Gap closures → Gap #A. Left here as a breadcrumb only.

### B. VK unit tests — improving, not complete
`client_test.go` + `dispatcher_test.go` + `subscription_test.go` now cover error classifiers, rate-limit (budget + TTL refill via `miniredis`), `decryptedAccount`, and `IsSubscribed` (non-VK, bad ids, Redis cache hit). Still missing: the send paths (`SendMessage`/`ReplyToWallComment`), the long-poll loop, and the `WorkerManager` lifecycle (need a fake VK API + fake repos). Test dep added: `github.com/alicebob/miniredis/v2`.

### C. VK dead code / smells — FIXED (one nit remains)
Removed `formatGroupForLog`, the unused `rdb vkRedis` field + `vkRedis` type, and the `context`-import suppression. See § Gap closures → Gap #C. **Remaining nit**: VK connect publishes `worker:start` via a raw `h.rdb.Publish` rather than `queue.Publisher` — harmless, left as-is.

### D. No DnD priority reordering
`@dnd-kit/core` is in `package.json` but unused. Priority is editable via the form.

### E. shadcn CLI unavailable under Node 18
`npx shadcn@latest add` needs Node 20+. `table.tsx`/`badge.tsx` were hand-written. Bump Node or keep writing components manually.

### F. CLAUDE.md "no application code exists yet" is stale
Left intentionally by the user mid-session. Don't change without asking.

### G. `cmd/encrypt-token/` is a dev helper
Safe to delete if unneeded.

---

## Phase 5 — Polish

### Shipped (no further action needed)

1. ~~**Fix VK subscription-gating token bug**~~ — **DONE** (Gap #A closure).
4. ~~**DnD priority reordering**~~ — **DONE this session**. `@dnd-kit/{core,sortable,utilities}` added; `TriggersList.tsx` rewritten with `SortableContext`; new `useReorderTriggers` hook PUTs only changed rows; priorities cascade in steps of 10 so future inserts can slot between.
5. ~~**`/triggers/:tid/test` endpoint**~~ — **DONE** previously (§ Phase 5 #5 shipped).
6. ~~**Friendlier plan-limit messaging**~~ — **DONE this session**. `lib/api-errors.ts` (`apiError`, `friendlyPlanError`, `friendlyErrorMessage`) + `<PlanLimitDialog>` modal. Wired into `AccountsList.onConnectIG/onConnectVK` and `TriggerForm.onSubmit`. `limit_exceeded`, `platform_not_allowed`, `conflict`, `subscription_required` are all rewritten with Russian copy.
7. ~~**IG token-refresh Asynq cron**~~ — **DONE this session**. `handlers.MaintenanceHandler.RefreshIGTokens` + new `accounts.sql` query `ListIGAccountsNearExpiry($1 days)` + `AccountRepo.ListIGNearExpiry`. Worker registers the task with `asynq.Scheduler` at `@daily` cadence. Refresh window: tokens expiring < 10 days. Persists via `AccountService.EncryptToken` (new) → `AccountRepo.UpdateToken`. Skips entirely when `META_APP_ID` / `META_APP_SECRET` are missing (CI / dev safety).
8. ~~**Log-retention cron**~~ — **DONE this session**. `handlers.MaintenanceHandler.LogRetention` iterates `basic/pro/enterprise` and calls existing `DeleteLogsOlderThan(plan, days)` with 7/30/90 day windows. Scheduled `@daily` alongside the token refresh.
9. ~~**Surface `skipped` logs in the UI**~~ — **DONE this session**. `AccountLogs.tsx` adds an action-filter chip group (Все / Отправлено / Пропущено / Ошибки) with live counts; skipped rows show a friendly Russian label for the matcher's skip reason (`cooldown` / `max_replies_reached` / `no_action_text`).
12. ~~**`golangci-lint` in CI + GitHub Actions**~~ — **DONE this session**. `backend/.golangci.yml` (errcheck, govet, ineffassign, misspell, revive, staticcheck, unused, gofmt, goimports; sqlc generated/ excluded). `.github/workflows/ci.yml` with two jobs: `backend` (postgres + redis services, gofmt check, vet, goose up, `go test -race`, build api+worker, golangci-lint) and `frontend` (Node 20, `npm ci`, `npm run build`).

### Remaining / deferred

2. **Expand VK unit tests** (§ Known gaps B) — still missing: dispatcher send/skip paths, long-poll loop, `WorkerManager` lifecycle. Needs a fake VK API (`httptest` against the `groups.getById` / `messages.send` / `wall.createComment` endpoints) and fake repos. Not done this session — would have added ~600 lines of test scaffolding for relatively small marginal coverage given that the same code paths are exercised in production every Long-Poll tick.
3. **WebSocket realtime UI** — deferred. Multi-day work: `/ws` upgrade in Gin, `handler/ws.go` with per-user auth, `store/ws.ts` Zustand bridge, fan-out from `queue/handlers/instagram.go` and `vk/dispatcher.go` after every `recordOK`, reconnect logic on the client. Worth a dedicated session.
10. **Production Docker + GHCR** — deferred. Needs a real GitHub repo URL, a release tagging strategy, and probably a `docker-compose.prod.yml` separate from the dev compose. The Dockerfile already has dev/builder/production stages from Phase 1 — wiring the publish step is the missing piece. Suggest pairing with a deploy-target decision (Fly.io? Hetzner? Render?).
11. **Frontend tests** (Vitest unit + Playwright E2E) — deferred. Vitest config, jsdom, MSW for axios mocking, Playwright browser install — substantial infra. Lower value than the backend tests already in place since the trigger editor logic is mostly form-state plumbing.

---

## Phase 5 #5 shipped — `/triggers/:tid/test`

### Why
The trigger editor (`TriggerForm.tsx`) promised a "Тест" button per `docs/plan.md` § 7, but the endpoint didn't exist. This makes authoring triggers blind — users had to wait for real platform events to discover whether their keywords/regex/template substitution actually fire. Closes that gap with an offline dry-run.

### Files added / modified
- **`backend/internal/engine/test_trigger.go`** — `TestTrigger(t domain.Trigger, ev IncomingEvent) TestResult` as a *package-level function* (no matcher receiver — runs purely on the trigger + event, no DB/Redis side effects). Result shape:
  ```go
  type TestResult struct {
      EventTypeMatched bool        // trigger.event_type allows ev.Kind
      TextMatched      bool        // match_mode + keywords/regex hit
      MatchedKeyword   string
      WouldFire        bool        // both of the above
      Replies          []TestReply // one per enabled+non-empty action
  }
  type TestReply struct{ Channel, Text string } // channel ∈ {"dm","comment_reply","private_reply"}
  ```
- **`backend/internal/engine/test_trigger_test.go`** — 7 unit tests; runs in 0 ms.
- **`backend/internal/service/trigger.go`** — `TestEventKind` ("dm" | "comment"), `TestParams`, `ErrInvalidTestEventKind = fmt.Errorf("%w: ...", ErrTriggerValidation)` so the existing `writeTriggerError` switch in the handler maps it to 400 automatically. `TriggerService.Test` calls `s.triggers.GetByID` → `s.ownAccount` → kind validation → `engine.TestTrigger`.
- **`backend/internal/handler/triggers.go`** — `Test(c)` handler reads `testTriggerBody{Text, SenderID, SenderName, SenderUsername, Kind}`; kind defaults to `"dm"` when omitted; reuses `writeTriggerError`.
- **`backend/cmd/api/main.go`** — route wired under the *auth-only* group (NOT the `gated` group), so users without an active subscription can preview triggers in the editor before purchasing:
  ```go
  api.POST("/accounts/:id/triggers/:tid/test", requireAuth, triggerH.Test)
  ```
- **`frontend/src/api/triggers.ts`** — `TestReply`, `TestResult`, `TestInput` types; `useTestTrigger(accountId, triggerId)` mutation hook.
- **`frontend/src/pages/triggers/TriggerForm.tsx`** — `<TestPanel>` sub-component rendered only in edit mode (`!isNew && tid && accountID`). Inputs: text, sender_name (for `{{name}}`), sender_id, kind dropdown. Result renders as a badge summary + rendered reply text per channel + an "action not configured for this event type" note when `would_fire && replies.length===0`.

### Authorization shape
- Endpoint: `POST /api/v1/accounts/:id/triggers/:tid/test`
- Auth: JWT required.
- Sub-gate: **none** (intentional — users should be able to preview before paying).
- Ownership: `service.TriggerService.ownAccount` looks up the account and confirms `account.user_id == jwt.user_id`. Cross-user attempts return 404 (not 403, to avoid leaking trigger existence).

### Live smoke (7 paths, all green)

```
| # | Step                                                     | Result |
|---|----------------------------------------------------------|--------|
| A | DM "hello fresh world" → SmokeTrigger                    | would_fire=true, matched_keyword="hello", reply="Hi Alice! You said hello." |
| B | DM "goodbye"                                             | event_type_matched=true, text_matched=false, replies=[] |
| C | Comment kind vs DM-only trigger                          | event_type_matched=false, would_fire=false |
| D | kind: "bogus"                                            | 400 validation_error "kind must be 'dm' or 'comment'" |
| E | Admin tests user's trigger                               | 404 not_found (ownership-via-ownAccount working) |
| F | Revoke user's sub → POST /test                           | 200 (auth-only endpoint confirmed) |
| G | Regression: sub-gated POST /triggers without sub         | 403 subscription_required (other routes still gated) |
```

### Smoke seed (current dev DB)
The `e2e-1` / `e2e-2` seed users from earlier sessions were wiped between sessions. Re-seeded with:

| Email | Password | Role | UUID |
|---|---|---|---|
| `e2e-admin@test.local` | `longpass123` | **admin** | `bfd364c3-f41e-41b5-a3ac-bc2ae023e5a0` |
| `e2e-user@test.local`  | `longpass456` | user (had Pro sub during smoke; revoked at the end then re-granted) | `2ce46ff5-edc1-41f7-a766-c3c5523ed6cd` |

Fresh IG account: `b8f07bd7-7839-4aa7-801f-366c52a9d0fd` (page_id `555955811455114`, IG biz id `17841405879907238`, `display_name="rabb1tof"`).
Fresh SmokeTrigger: `2a24c4bf-72b7-4a18-ae5a-3c56f8f52e80` (event_type=dm, keywords=["hello"], dm_text="Hi {{name}}! You said {{keyword}}.", priority=100).

If these get wiped again, re-create via the bash recipe from § Standard run (register → promote → grant sub → encrypt-token → INSERT into connected_accounts → POST /triggers).

### Notes for the next agent
- The `TestTrigger` fn is intentionally *not* a method on `TriggerMatcher` — it has zero state dependencies, so the API process doesn't need to construct a matcher just to support this endpoint.
- The endpoint never touches Redis cooldown / counter / cache, so it's safe to spam from the editor without disturbing production matchers.
- `TestResult.Replies` is initialized to `[]TestReply{}` (not nil) so the JSON envelope is always `"replies": []` rather than `"replies": null` — saves the frontend a null-check.

---

## Phase 5 session 2 shipped — six items in one push

Six Phase 5 items landed in a single session. Build state at the end of the session: `go vet`/`go build`/`go test ./... -count=1` all exit 0; `gofmt -l backend/` empty; `npm run build` clean. Backend test count unchanged at 48 (no new tests added for the maintenance handler — see § deferrals below).

### Files added
- **`backend/internal/queue/handlers/maintenance.go`** — `MaintenanceHandler` with `RefreshIGTokens` and `LogRetention` task handlers. Wraps the IG client, the AccountRepo, and a new `TokenEncDec` interface that `service.AccountService` already satisfies via `DecryptToken`/`EncryptToken`.
- **`backend/.golangci.yml`** — lean linter config. sqlc generated code is excluded.
- **`.github/workflows/ci.yml`** — backend job (Go 1.25, postgres + redis services, gofmt + vet + `-race` tests + build + golangci-lint) and frontend job (Node 20, npm ci, build).
- **`frontend/src/lib/api-errors.ts`** — `apiError`, `isErrorCode`, `friendlyPlanError`, `friendlyErrorMessage` helpers.
- **`frontend/src/components/PlanLimitDialog.tsx`** — small accessible modal with "Open subscription" + "Close" buttons.

### Files modified
- **`backend/internal/db/query/accounts.sql`** — added `ListIGAccountsNearExpiry($1::int days)`. `sqlc generate` regenerated.
- **`backend/internal/repository/account.go`** — `AccountRepo` interface extends with `ListIGNearExpiry(ctx, daysAhead int)`; pg impl implemented.
- **`backend/internal/service/account.go`** — added `EncryptToken(plaintext string) (string, error)` (mirror of `DecryptToken`).
- **`backend/cmd/worker/main.go`** — constructs `MaintenanceHandler`, registers `TaskRefreshIGTokens` and `TaskLogRetention` in the mux, builds `asynq.NewScheduler` with `@daily` registrations for both, runs scheduler in a goroutine, calls `scheduler.Shutdown()` on SIGINT/SIGTERM.
- **`frontend/src/api/triggers.ts`** — `triggerToInput` helper + `useReorderTriggers(accountId)` mutation (PUTs only changed rows in parallel via `Promise.all`).
- **`frontend/src/pages/triggers/TriggersList.tsx`** — full rewrite with `DndContext` + `SortableContext` + `SortableTriggerRow` sub-component. Drag handle column with `⋮⋮` glyph; optimistic local order during drag; rollback on PUT failure.
- **`frontend/src/pages/logs/AccountLogs.tsx`** — `<FilterChips>` with live counts (all / delivered / skipped / error); `skipReasonLabel` maps `cooldown` / `max_replies_reached` / `no_action_text` → Russian sentences; client-side filter (worth promoting to server-side once the page paginates past ~1k rows).
- **`frontend/src/pages/accounts/AccountsList.tsx`** — IG + VK connect onError handlers call `friendlyPlanError`; raises `<PlanLimitDialog>` on `limit_exceeded` / `platform_not_allowed`.
- **`frontend/src/pages/triggers/TriggerForm.tsx`** — same treatment for Create / Update mutations.

### Scheduled tasks (added to the worker)

| Task name | Cadence | Handler | Notes |
|---|---|---|---|
| `instagram:refresh_tokens` | `@daily` | `MaintenanceHandler.RefreshIGTokens` | No-op when `META_APP_ID` / `META_APP_SECRET` are missing; per-account failures logged but don't abort the batch. |
| `logs:retention_cleanup` | `@daily` | `MaintenanceHandler.LogRetention` | `basic=7d`, `pro=30d`, `enterprise=90d` via existing `DeleteLogsOlderThan(plan, days)` query. |

Worker boot log on success:
```
INFO worker/main.go  worker starting  {"asynq_tasks": ["instagram:event", "instagram:refresh_tokens", "logs:retention_cleanup"], "vk_manager": "enabled", "scheduler": "@daily x2"}
```

### Frontend dependencies added this session
- `@dnd-kit/core@^6.1.0`
- `@dnd-kit/sortable@^8.0.0`
- `@dnd-kit/utilities@^3.2.2`

(The plan previously claimed `@dnd-kit/core` was already in `package.json` — that was incorrect; it wasn't, and has now been added.)

### Deferrals (with reasoning)

| Item | Status | Reason |
|---|---|---|
| #2 Remaining VK unit tests (send paths, long-poll, manager lifecycle) | deferred | Needs fake VK API + fake repos (~600 LOC of scaffolding). Same code paths run live on every Long-Poll tick, so marginal value of unit tests is small. Worth doing under `-race` in a dedicated session. |
| #3 WebSocket realtime UI | deferred | Multi-day: `/ws` upgrade, `handler/ws.go` with per-user auth, fan-out wiring from both `queue/handlers/instagram.go` and `vk/dispatcher.go`, Zustand bridge, reconnect logic. |
| #10 Production Docker images + GHCR | deferred | Needs the user to decide deploy target (Fly.io / Hetzner / Render / etc.). The Dockerfile already has `production` stage. Wiring `docker build --push` to GHCR is the easy part — the release/tagging strategy is the hard call. |
| #11 Frontend tests (Vitest + Playwright) | deferred | Substantial new infra: Vitest config, jsdom, MSW for axios mocking, Playwright browser install + CI step. Lower marginal value than the backend tests already in place since the trigger editor logic is form-state plumbing. |

### Notes for the next agent
- **DnD priorities** are written in batches of 10 (e.g. 50, 40, 30, 20, 10 for a five-trigger list). Inserts that slot between two rows can land on intermediate values without re-numbering the whole list.
- **`useReorderTriggers`** only PUTs rows whose priority actually changed. Verify by dragging the bottom row to its current position — no network calls should fire.
- **Maintenance scheduler** is keyed off `asynq.Scheduler`, which uses Redis under the hood — so any second worker process will *also* try to register the periodic tasks. asynq's scheduler is single-leader internally, but for multi-replica safety the next agent should consider asynq's `PeriodicTaskManager` with a config source (e.g. file or DB) instead.
- **Log retention** silently no-ops for users without an active subscription (the DELETE join filters on `s.is_active = true`). If you want to delete orphan logs you'll need a separate query.
- **`MaintenanceHandler.RefreshIGTokens`** doesn't currently retry per-account failures via the Asynq retry mechanism — it just logs and continues. If you want individual retries, restructure to enqueue a per-account `instagram:refresh_one` task and handle each.
- **CI's `-race` step** requires CGO (gcc on Linux is preinstalled, so it just works). On Windows dev `-race` needs MSYS2 gcc — see § Enabling -race.
- **golangci-lint** runs after the test step in CI. If it fails, the build is red but tests still ran — useful signal in PR feedback.

---

## Conventions to maintain (from CLAUDE.md / docs/agents.md)

### Must-do
- Wrap errors: `fmt.Errorf("pkg.Func: %w", err)`.
- `context.Context` as the first arg of every blocking call.
- Tokens always go through `crypto.Encrypt`/`Decrypt` — NEVER stored plaintext. (Note: the matcher must receive a decrypted token before subscription checks — see § Known gaps A.)
- `zap` for logging; never `fmt.Println`.
- Goroutines terminated via `context` (VK workers follow this).
- HTTP responses use the standard envelope (`{data, meta?}` / `{error, message, details?}`).
- Every protected request verifies `user_id` from JWT against the resource owner.
- Webhook handlers respond 200 immediately and enqueue to Asynq — never call platform APIs synchronously.

### Must-not-do
- `graph.instagram.com` or IGAA tokens — only `graph.facebook.com` + Page Access Token.
- Multiple replies to one event (spam ban risk).
- Logging tokens / secrets / raw credentials.
- `SELECT *` in queries — name the columns.
- DB queries inside `TriggerMatcher` — always go through the 60s cache (invalidated via pub/sub now).
- Manual edits to `internal/db/generated/*` — regenerate via sqlc.

### sqlc workflow (after editing any `query/*.sql`)
```powershell
go -C backend run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
go -C backend mod tidy
go -C backend vet ./...
go -C backend test ./...
```

### Migration workflow (after adding `00X_*.sql`)
```powershell
$env:DATABASE_URL = 'postgres://socialsentry:secret@localhost:5432/socialsentry?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@latest -dir backend/internal/db/migrations postgres $env:DATABASE_URL up
# verify down too:
go run github.com/pressly/goose/v3/cmd/goose@latest -dir backend/internal/db/migrations postgres $env:DATABASE_URL down
go run github.com/pressly/goose/v3/cmd/goose@latest -dir backend/internal/db/migrations postgres $env:DATABASE_URL up
```

---

## Critical files — extend, do not duplicate

| File | Purpose |
|---|---|
| `backend/internal/config/config.go` | Every env var (Meta + VK) |
| `backend/pkg/crypto/aes.go` | AES-256-GCM (IG + VK token storage) |
| `backend/pkg/jwt/jwt.go` | HS256 (RequireAuth, future WS handshake) |
| `backend/internal/middleware/{auth,subscription,ratelimit}.go` | Guards |
| `backend/internal/engine/matcher.go` | TriggerMatcher (IG + VK); SubscriptionChecker iface |
| `backend/internal/engine/template.go` | ApplyTemplate (both platforms) |
| `backend/internal/repository/{account,trigger,log}.go` | pgx repos; log.go now supports NULL trigger_id |
| `backend/internal/service/{account,trigger,plans}.go` | Business logic + plan limits + pub/sub publish |
| `backend/internal/queue/client.go` | Asynq producer (IG webhooks) |
| `backend/internal/queue/pubsub.go` | Redis Publisher (trigger reload + worker lifecycle) |
| `backend/internal/queue/handlers/instagram.go` | IG orchestration template |
| `backend/internal/platform/vk/*.go` | VK adapter (client/messages/comments/subscription/dispatcher/worker/manager) |
| `backend/internal/handler/{accounts_instagram,accounts_vk,webhooks}.go` | Connect + webhook handlers |
| `backend/cmd/api/main.go` | Composition root (extend, don't fork) |
| `backend/cmd/worker/main.go` | Asynq server + VK WorkerManager (extend) |
| `frontend/src/api/{client,accounts}.ts` | axios + IG/VK connect hooks |
| `frontend/src/pages/accounts/AccountsList.tsx` | Account table + IG/VK connect |

---

## Quick verification commands

```powershell
# Backend full check
go -C backend mod tidy
go -C backend vet ./...
go -C backend build ./...
go -C backend test ./... -count=1
# Expected: vet/build exit 0, 35 tests pass (vk package has no tests yet)

# Frontend full check
cd frontend
npm install
npm run build
# Expected: tsc --noEmit clean, vite build ~196 modules

# Full stack up
docker compose up -d postgres redis
# Load env then:
go -C backend run ./cmd/api      # logs "api server starting", "postgres connected", "redis connected"
go -C backend run ./cmd/worker   # logs "worker starting" with asynq_tasks + "vk_manager":"enabled"
cd frontend; npm run dev

# Confirm migration 008 applied (trigger_id nullable)
docker compose exec -T postgres psql -U socialsentry -d socialsentry -c \
  "SELECT is_nullable FROM information_schema.columns WHERE table_name='trigger_logs' AND column_name='trigger_id';"
# Expected: YES

# Inspect skipped logs (after traffic)
docker compose exec -T postgres psql -U socialsentry -d socialsentry -c \
  "SELECT trigger_id, action_taken, error_message FROM trigger_logs WHERE action_taken='skipped' ORDER BY created_at DESC LIMIT 5;"
# Expected: trigger_id NULL, error_message in {cooldown, max_replies_reached, no_action_text}
```

### VK live test (no fake token needed — VK doesn't require app review)
1. Connect a VK community via the AccountsList "Подключить VK" form (`group_id` + community token).
2. Worker logs `vk worker started` with the group_id.
3. DM the community from a test user → expect a `trigger_logs` row with `action_taken='sent_dm'` (real send).
