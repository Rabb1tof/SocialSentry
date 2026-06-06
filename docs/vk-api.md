# VK API — Руководство по интеграции

> VK интеграция значительно проще Instagram:  
> никакого App Review, токен генерируется за 5 минут, работает из РФ без ограничений.

---

## Ключевые выводы

| Факт | Детали |
|------|--------|
| **SDK** | vksdk v3 (github.com/SevereCloud/vksdk/v3) |
| **Метод получения событий** | Bots Long Poll API (pull) |
| **Токен** | Community Token из настроек группы |
| **App Review** | Не нужен |
| **Проверка подписки** | `groups.isMember` — быстро и бесплатно |

---

## Получение Community Token

1. VK → Управление сообществом → Работа с API → Ключи доступа
2. Создать ключ с правами:
   - `messages` — отправка сообщений
   - `wall` — ответы на комментарии стены
   - `manage` — управление сообществом
3. Включить Bots Long Poll API: Управление → Настройки → Работа с API → Long Poll API → Включить

Токен хранится в БД в зашифрованном виде (AES-256).

---

## Подключение аккаунта (со стороны пользователя)

В отличие от Instagram OAuth — пользователь просто вставляет токен и group_id в форму. Никаких редиректов.

```json
POST /api/v1/accounts/vk/connect
{
  "group_id": "123456789",
  "community_token": "vk1.a.xxxx..."
}
```

Бэкенд верифицирует токен запросом к VK API и запускает воркер.

---

## VK Worker (Long Poll)

```go
// platform/vk/worker.go
type VKWorker struct {
    vk        *api.VK
    lp        *longpoll.LongPoll
    accountID string
    groupID   int
    engine    *engine.TriggerEngine
    ctx       context.Context
    cancel    context.CancelFunc
}

func NewVKWorker(token string, groupID int, accountID string, eng *engine.TriggerEngine) *VKWorker {
    vk := api.NewVK(token)
    lp, _ := longpoll.NewLongPoll(vk, groupID)

    ctx, cancel := context.WithCancel(context.Background())
    return &VKWorker{
        vk: vk, lp: lp,
        accountID: accountID, groupID: groupID,
        engine: eng, ctx: ctx, cancel: cancel,
    }
}

func (w *VKWorker) Run() {
    // Входящее сообщение в ЛС сообщества
    w.lp.MessageNew(func(_ context.Context, obj events.MessageNewObject) {
        w.engine.ProcessDM(w.ctx, w.accountID, &DMEvent{
            SenderID: obj.Message.FromID,
            Text:     obj.Message.Text,
            Platform: "vk",
        })
    })

    // Новый комментарий на стене
    w.lp.WallReplyNew(func(_ context.Context, obj events.WallReplyNewObject) {
        w.engine.ProcessComment(w.ctx, w.accountID, &CommentEvent{
            SenderID:  obj.FromID,
            CommentID: obj.ID,
            PostID:    obj.PostID,
            Text:      obj.Text,
            Platform:  "vk",
        })
    })

    // Запуск (блокирующий, выходит при cancel)
    if err := w.lp.Run(); err != nil && w.ctx.Err() == nil {
        log.Printf("VK worker %s error: %v", w.accountID, err)
    }
}

func (w *VKWorker) Stop() {
    w.cancel()
}
```

---

## Отправка сообщений

### Ответ в ЛС

```go
func (c *VKClient) SendMessage(ctx context.Context, userID int, text string) error {
    _, err := c.vk.MessagesSend(api.Params{
        "user_id":   userID,
        "message":   text,
        "random_id": time.Now().UnixNano(), // обязательный уникальный ID
    })
    return err
}
```

### Ответ на комментарий стены

```go
func (c *VKClient) ReplyToComment(ctx context.Context, ownerID, postID, commentID int, text string) error {
    _, err := c.vk.WallCreateComment(api.Params{
        "owner_id":           ownerID,
        "post_id":            postID,
        "reply_to_comment":   commentID,
        "message":            text,
    })
    return err
}
```

---

## Проверка подписки на группу

```go
func (c *VKClient) IsMember(ctx context.Context, groupID, userID int) (bool, error) {
    result, err := c.vk.GroupsIsMember(api.Params{
        "group_id": groupID,
        "user_id":  userID,
    })
    if err != nil {
        return false, err
    }
    return result == 1, nil
}
```

Кэшировать результат в Redis (TTL 5 минут) чтобы не дёргать API на каждое сообщение.

---

## Доступные события Long Poll

| Событие | Описание |
|---------|---------|
| `MessageNew` | Новое сообщение в ЛС сообщества |
| `WallReplyNew` | Новый комментарий на стене |
| `WallReplyEdit` | Комментарий отредактирован |
| `WallReplyDelete` | Комментарий удалён |
| `PhotoCommentNew` | Новый комментарий к фото |
| `VideoCommentNew` | Новый комментарий к видео |
| `MarketCommentNew` | Новый комментарий к товару |

Для MVP достаточно `MessageNew` + `WallReplyNew`.

---

## Лимиты VK API

| Лимит | Значение |
|-------|---------|
| Запросов к API | 20 / секунду на токен |
| MessagesSend | Нельзя слать одному пользователю чаще чем раз в секунду |
| random_id | Должен быть уникальным (иначе дубликат не отправится) |

**Rate-limiter для VK:**

```go
// Leaky bucket — 20 rps
type VKRateLimiter struct {
    ticker *time.Ticker
    tokens chan struct{}
}

func NewVKRateLimiter() *VKRateLimiter {
    rl := &VKRateLimiter{
        ticker: time.NewTicker(50 * time.Millisecond), // 20 rps
        tokens: make(chan struct{}, 20),
    }
    go func() {
        for range rl.ticker.C {
            select {
            case rl.tokens <- struct{}{}:
            default:
            }
        }
    }()
    return rl
}

func (rl *VKRateLimiter) Wait() {
    <-rl.tokens
}
```

---

## Верификация Callback API (альтернатива Long Poll)

Если нужен Callback API вместо Long Poll:

```go
// POST /webhooks/vk/:group_id
func HandleVKCallback(c *gin.Context) {
    var event VKCallbackEvent
    c.BindJSON(&event)

    // Подтверждение сервера при настройке
    if event.Type == "confirmation" {
        c.String(200, cfg.VK.ConfirmationToken)
        return
    }

    // Проверка secret
    if event.Secret != cfg.VK.CallbackSecret {
        c.Status(403)
        return
    }

    // Немедленный ответ ok
    c.String(200, "ok")

    // Асинхронная обработка
    go queue.EnqueueVKEvent(event)
}
```

---

## Получение информации о пользователе

```go
func (c *VKClient) GetUser(ctx context.Context, userID int) (*VKUser, error) {
    users, err := c.vk.UsersGet(api.Params{
        "user_ids": userID,
        "fields":   "first_name,last_name,photo_50",
    })
    if err != nil || len(users) == 0 {
        return nil, err
    }
    u := users[0]
    return &VKUser{
        ID:        u.ID,
        FirstName: u.FirstName,
        LastName:  u.LastName,
        Avatar:    u.Photo50,
    }, nil
}
```

Используется для переменной `{{name}}` в шаблонах ответов.
