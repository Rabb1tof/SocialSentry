# Подписки, тарифные планы и просмотр логов

> Справочник по системе подписок SocialSentry: чем различаются планы, где это
> применяется в коде, что доступно без подписки и **где пользователь смотрит логи**.
> Единственный источник правды по лимитам — [`backend/internal/service/plans.go`](../backend/internal/service/plans.go).

---

## Общая модель

- Подписки **выдаёт только админ** вручную (`POST /api/v1/admin/subscriptions`). Публичного
  self-service и оплаты нет.
- Подписка считается активной, если: запись существует, `is_active = true`, и
  `expires_at IS NULL` (бессрочно) **или** `expires_at > now()`.
- Без активной подписки аккаунт работает в режиме «только чтение»: просмотр доступен,
  но любые мутации (подключение аккаунтов, создание/изменение триггеров, просмотр логов
  триггера) заблокированы middleware `RequireActiveSubscription` → `403 subscription_required`.

---

## Тарифные планы

Определены в [`service/plans.go`](../backend/internal/service/plans.go) (`PlanLimitsByName`).
`-1` = без лимита.

| Параметр | **basic** | **pro** | **enterprise** |
|---|---|---|---|
| Макс. аккаунтов | **2** | **10** | **∞** |
| Макс. триггеров на аккаунт | **5** | **50** | **∞** |
| Хранение логов | **7 дней** | **30 дней** | **90 дней** |
| Несколько платформ сразу | **нет** (только IG **или** VK) | **да** | **да** |
| Доступные платформы | IG + VK | IG + VK | IG + VK |

Неизвестный/незаданный план → лимиты **basic** (`default` в `PlanLimitsByName`).

### Нюансы
- **basic — одна платформа за раз** (`MultiplePlatforms: false`): если уже подключён аккаунт
  одной платформы, аккаунт другой подключить нельзя. У pro/enterprise — любой микс.
- **Платформы не запрещаются тарифом** — `AllowedPlatforms` у всех `{vk, instagram}`. Разница
  только в возможности их совмещать.
- Это **не** связано с админским «рубильником» платформ (`platform_settings`): тариф — это
  лимиты пользователя, а рубильник — глобальное отключение платформы админом. См.
  [`database.md`](./database.md) (таблица `platform_settings`).

### Чего планы НЕ различают
Функционал одинаков на всех тарифах: DM, комментарии, приватные ответы (DM по комментарию),
режимы совпадения (keyword/all/regex), шаблоны `{{name}}`/`{{keyword}}` и т.д. Разница
**только** в квотах, сроке хранения логов и совмещении платформ.

---

## Где лимиты применяются (в коде)

| Лимит | Где enforce'ится |
|---|---|
| Кол-во аккаунтов + правило «одна платформа» | [`service/account.go` → `checkPlanLimits`](../backend/internal/service/account.go) (при подключении аккаунта) |
| Кол-во триггеров на аккаунт | [`service/trigger.go`](../backend/internal/service/trigger.go) (при создании триггера) |
| Хранение логов | ежедневная Asynq-задача [`queue/handlers/maintenance.go`](../backend/internal/queue/handlers/maintenance.go) удаляет логи старше N дней по плану |

> ⚠️ **Дубль источника правды:** срок хранения логов сейчас задан в двух местах —
> `PlanLimits.LogRetentionDays` (plans.go) и хардкод-список в `maintenance.go`
> (`basic:7, pro:30, enterprise:90`). Значения совпадают, но при изменении правьте оба
> (лучше — свести `maintenance.go` к `PlanLimitsByName(...).LogRetentionDays`).

---

## Доступ без активной подписки

**Заблокировано** (`403 subscription_required`):
- `POST /accounts/{instagram,vk}/connect` — подключение аккаунтов
- `POST/PUT/DELETE /accounts/:id/triggers/*` — создание/изменение/удаление триггеров
- `GET /accounts/:id/logs` и `GET /accounts/:id/triggers/:tid/logs` — просмотр логов
- запуск воркеров (WorkerManager проверяет подписку)

**Доступно без подписки**:
- `GET /accounts` — список подключённых аккаунтов
- `GET /accounts/:id/triggers` — просмотр триггеров (редактирование заблокировано)
- `GET /subscription` — свой статус подписки
- `GET /logs/recent` — лента последней активности (см. ниже)

---

## Где пользователь смотрит логи

Логи срабатываний (`trigger_logs`) пользователю доступны в **двух местах**:

1. **Дашборд → «Последняя активность»** — лента последних 20 срабатываний по всем
   аккаунтам пользователя. Эндпоинт `GET /api/v1/logs/recent`
   ([`handler/triggers.go` → `RecentLogs`](../backend/internal/handler/triggers.go),
   репозиторий `LogRepo.ListByUser`). **Auth-only** — показывается даже без активной
   подписки. Обновляется в реальном времени по WebSocket
   ([`frontend/src/lib/realtime.ts`](../frontend/src/lib/realtime.ts) инвалидирует
   `["logs","recent"]`). Фронт: [`pages/dashboard/Dashboard.tsx`](../frontend/src/pages/dashboard/Dashboard.tsx).

2. **Страница логов аккаунта** — полная таблица с фильтрами (Все / Отправлено /
   Пропущено / Ошибки) и пагинацией. Маршрут `/accounts/:id/logs`
   ([`pages/logs/AccountLogs.tsx`](../frontend/src/pages/logs/AccountLogs.tsx)),
   эндпоинт `GET /api/v1/accounts/:id/logs`. **Требует активную подписку** — без неё
   API вернёт `403`, и axios-интерсептор перенаправит на `/subscription`.
   Ссылка «Логи» есть в строке аккаунта ([`pages/accounts/AccountsList.tsx`](../frontend/src/pages/accounts/AccountsList.tsx))
   и в шапке страницы триггеров ([`pages/triggers/TriggersList.tsx`](../frontend/src/pages/triggers/TriggersList.tsx)).

### Что видно в логе
Каждая запись: время, аккаунт/платформа, тип события (`dm`/`comment`/…), отправитель,
входящий текст, сработавший ключ, действие (`sent_dm` / `replied_comment` / `both` /
`skipped` / `error`) и текст ошибки/причина пропуска. Структура — `domain.TriggerLog`
([`backend/internal/domain/log.go`](../backend/internal/domain/log.go)).

### Срок хранения
Пользователь видит только логи **в пределах retention своего плана** (7 / 30 / 90 дней) —
более старые удаляются ежедневной maintenance-задачей. Есть только серверная пагинация по
времени; фильтр по действию пока клиентский.

> Существует и эндпоинт логов по конкретному триггеру
> (`GET /accounts/:id/triggers/:tid/logs`, требует подписку), но отдельной страницы под него
> на фронте сейчас нет.

---

## Как менять

- **Лимиты планов** — [`service/plans.go`](../backend/internal/service/plans.go) (`PlanLimitsByName`).
- **Срок хранения логов** — там же (`LogRetentionDays`) **и** дубль в
  [`maintenance.go`](../backend/internal/queue/handlers/maintenance.go) — синхронизируйте.
- **Названия планов** — константы в [`domain/subscription.go`](../backend/internal/domain/subscription.go)
  (`PlanBasic`/`PlanPro`/`PlanEnterprise`); фронт-енам — в
  [`frontend/src/api/subscription.ts`](../frontend/src/api/subscription.ts).
- **Ввести функциональные различия между планами** (а не только квоты) — добавьте поля в
  `PlanLimits` и проверяйте их в соответствующих сервисах/движке.
