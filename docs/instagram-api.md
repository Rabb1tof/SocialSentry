# Instagram API — Practical Guide

> Everything below has been **verified in practice** during a live testing session.
> App ID: `1388425723092623` (SocialSentry-IG)

---

## Key Facts

| Fact | Detail |
|------|--------|
| **Domain** | `graph.facebook.com` — for all operations |
| **Token** | Page Access Token (`EAAZAjwU...`) — one token for everything |
| **Auth method** | `?access_token={token}` query param OR body field |
| **IG Business Account ID** | `17841405879907238` |
| **Facebook Page ID** | `555955811455114` |
| **Dev Mode limit** | API only sees conversations with users who have a role in the Meta App |
| **Initiating DMs** | Not allowed — can only reply to incoming messages (24h window) |

---

## Token Types — Only One Needed

### Page Access Token (`EAAZAjwU...`)
- Obtained via `GET /me/accounts` with a Facebook User Access Token
- Used for **all operations**: sending DMs, reading conversations, replying to comments
- Lives ~60 days — must be refreshed on day 50
- Stored encrypted (AES-256-GCM) in the DB

There is no need for Instagram-specific IGAA tokens in this project.
Everything works through `graph.facebook.com` + Page Access Token.

---

## OAuth Flow (connecting an account)

### Step 1 — Generate auth URL

```
https://www.facebook.com/dialog/oauth
  ?client_id={META_APP_ID}
  &redirect_uri={META_CALLBACK_URL}
  &scope=instagram_business_basic,instagram_business_manage_messages,
         instagram_business_manage_comments,pages_show_list,
         pages_read_engagement,pages_manage_metadata,pages_messaging
  &response_type=code
  &state={random_csrf_token}
```

### Step 2 — Callback: exchange code for short-lived User token

```
GET https://graph.facebook.com/v21.0/oauth/access_token
  ?client_id={META_APP_ID}
  &client_secret={META_APP_SECRET}
  &redirect_uri={META_CALLBACK_URL}
  &code={code_from_callback}
```

Response:
```json
{
  "access_token": "EAAZAjwU...",
  "token_type": "bearer",
  "expires_in": 3600
}
```

### Step 3 — Exchange for long-lived User token (~60 days)

```
GET https://graph.facebook.com/v21.0/oauth/access_token
  ?grant_type=fb_exchange_token
  &client_id={META_APP_ID}
  &client_secret={META_APP_SECRET}
  &fb_exchange_token={short_lived_token}
```

### Step 4 — Get Page Access Token + Page ID

```
GET https://graph.facebook.com/v21.0/me/accounts
  ?access_token={long_lived_user_token}
```

Response:
```json
{
  "data": [{
    "access_token": "EAAZAjwU...",   ← Page Access Token (save this)
    "id": "555955811455114",          ← Facebook Page ID (save this)
    "name": "Artyom",
    "tasks": ["ADVERTISE", "ANALYZE", "CREATE_CONTENT", "MESSAGING", "MODERATE", "MANAGE"]
  }]
}
```

Verify that `MESSAGING` is present in `tasks`.

### Step 5 — Get Instagram Business Account ID

```
GET https://graph.facebook.com/v21.0/{page_id}
  ?fields=instagram_business_account
  &access_token={page_access_token}
```

Response:
```json
{
  "instagram_business_account": { "id": "17841405879907238" },
  "id": "555955811455114"
}
```

### Step 6 — Subscribe app to the page (REQUIRED)

Without this step the conversations API always returns empty data.

```
POST https://graph.facebook.com/v21.0/{page_id}/subscribed_apps
  ?subscribed_fields=messages,messaging_postbacks,messaging_optins
  &access_token={page_access_token}
```

Expected response:
```json
{ "success": true }
```

### Step 7 — Save to DB (encrypt token)

```
connected_accounts:
  platform         = 'instagram'
  platform_id      = '17841405879907238'  ← Instagram Business Account ID
  page_id          = '555955811455114'    ← Facebook Page ID
  access_token     = encrypt(page_access_token)
  token_expires_at = now() + 60 days
```

---

## Token Refresh

Page Access Token lives ~60 days. Asynq cron task `token_refresh` renews it on day 50:

```
GET https://graph.facebook.com/v21.0/oauth/access_token
  ?grant_type=fb_exchange_token
  &client_id={META_APP_ID}
  &client_secret={META_APP_SECRET}
  &fb_exchange_token={current_page_access_token}
```

---

## Sending Messages

### Reply to a DM ✅ VERIFIED

```bash
curl --location 'https://graph.facebook.com/v21.0/{page_id}/messages' \
--header 'Content-Type: application/json' \
--data '{
    "recipient":      {"id": "{sender_ig_scoped_id}"},
    "message":        {"text": "Hello!"},
    "messaging_type": "RESPONSE",
    "access_token":   "{page_access_token}"
}'
```

**Verified values:**
```
page_id             = 555955811455114
sender_ig_scoped_id = 1644146630022851  (zudina.anyaa in test session)
```

**Successful response:**
```json
{
  "recipient_id": "1644146630022851",
  "message_id":   "aWdfZAG1f..."
}
```

**Constraints:**
- `messaging_type: RESPONSE` — only within **24 hours** of the user's message
- Cannot initiate a conversation first — must reply to an incoming message
- Recipient must be the user who messaged you (their `sender.id` from the webhook)

### Reply to a Comment Publicly ✅ VERIFIED

Posts a visible reply in the comment thread.

```bash
curl --location --request POST \
  'https://graph.facebook.com/v21.0/{comment_id}/replies' \
--header 'Content-Type: application/json' \
--data '{
    "message":      "Thanks for your comment!",
    "access_token": "{page_access_token}"
}'
```

### Private Reply to a Comment (send DM from a comment) ✅ SUPPORTED

When a user leaves a comment on your post, reel, or live — you can send them
**one private DM** in response. This uses the same messages endpoint but with
`comment_id` as the recipient identifier instead of `id`.

```bash
curl --location 'https://graph.facebook.com/v21.0/{page_id}/messages' \
--header 'Content-Type: application/json' \
--data '{
    "recipient":      {"comment_id": "{comment_id}"},
    "message":        {"text": "Hey! Sent you more info in DM 📩"},
    "messaging_type": "RESPONSE",
    "access_token":   "{page_access_token}"
}'
```

**Key difference from regular DM:**

| | Regular DM reply | Private Reply from comment |
|--|-----------------|---------------------------|
| Recipient field | `{"id": "sender_id"}` | `{"comment_id": "comment_id"}` |
| Trigger | User sent you a DM | User left a comment |
| Window | 24 hours | 7 days from the comment |
| Messages allowed | Unlimited within window | One message; full dialog opens if user replies back |
| Endpoint | `/{page_id}/messages` | `/{page_id}/messages` (same) |

**Both actions can fire simultaneously from one trigger:**
1. Public reply in the comment thread → `POST /{comment_id}/replies`
2. Private DM to the commenter → `POST /{page_id}/messages` with `comment_id` recipient

This is the most powerful combo for engagement: publicly acknowledge the comment
AND send a private message with details, links, or offers.

---

## Reading Conversations ✅ VERIFIED

```bash
curl 'https://graph.facebook.com/v21.0/{page_id}/conversations
  ?platform=instagram
  &limit=5
  &fields=id,participants,updated_time
  &access_token={page_access_token}'
```

**Note:** In Development Mode only returns conversations with users
who have a role in the Meta App (Tester / Developer).

To get participants (sender IDs) from a conversation:

```bash
curl 'https://graph.facebook.com/v21.0/{page_id}/conversations
  ?platform=instagram
  &limit=1
  &fields=id,participants
  &access_token={page_access_token}'
```

Response:
```json
{
  "data": [{
    "id": "aWdfZAG06...",
    "participants": {
      "data": [
        {"username": "rabb1tof",     "id": "17841405879907238"},
        {"username": "zudina.anyaa", "id": "1644146630022851"}
      ]
    }
  }]
}
```

---

## Webhooks (Meta → our server)

### Verification (GET)

Meta sends a GET when configuring the webhook:

```
GET /webhooks/instagram
  ?hub.mode=subscribe
  &hub.verify_token={META_WEBHOOK_VERIFY_TOKEN}
  &hub.challenge=1234567
```

Respond with `hub.challenge` as plain text:

```go
func VerifyWebhook(c *gin.Context) {
    mode      := c.Query("hub.mode")
    token     := c.Query("hub.verify_token")
    challenge := c.Query("hub.challenge")

    if mode == "subscribe" && token == cfg.Meta.WebhookVerifyToken {
        c.String(200, challenge)
        return
    }
    c.Status(403)
}
```

### Signature Verification (mandatory on every POST)

```go
func verifySignature(body []byte, signature string) bool {
    mac := hmac.New(sha256.New, []byte(cfg.Meta.AppSecret))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

### Incoming DM Event Structure

```json
{
  "object": "instagram",
  "entry": [{
    "id": "555955811455114",
    "time": 1234567890,
    "messaging": [{
      "sender":    {"id": "1644146630022851"},
      "recipient": {"id": "17841405879907238"},
      "timestamp": 1234567890,
      "message": {
        "mid":  "aWdfZAG06...",
        "text": "Hello!"
      }
    }]
  }]
}
```

### Incoming Comment Event Structure

```json
{
  "object": "instagram",
  "entry": [{
    "id": "555955811455114",
    "changes": [{
      "field": "comments",
      "value": {
        "from": {"id": "1644146630022851", "username": "zudina.anyaa"},
        "media": {"id": "17841405879907238_123456", "media_product_type": "FEED"},
        "id":   "17858893269000001",
        "text": "Great post!"
      }
    }]
  }]
}
```

### Webhook Handler Pattern

```go
func HandleWebhook(c *gin.Context) {
    body, _ := io.ReadAll(c.Request.Body)
    signature := c.GetHeader("X-Hub-Signature-256")

    if !verifySignature(body, signature) {
        c.Status(403)
        return
    }

    // Respond immediately — Meta expects reply within 20 seconds
    c.Status(200)

    // Process asynchronously via Asynq
    go queue.EnqueueInstagramEvent(body)
}
```

---

## Required App Permissions

| Permission | Purpose | Status |
|-----------|---------|--------|
| `instagram_business_basic` | Basic profile info | Dev mode ✅ |
| `instagram_business_manage_messages` | Read/send DMs | Dev mode ✅ |
| `instagram_business_manage_comments` | Read/reply to comments | Dev mode ✅ |
| `instagram_business_content_publish` | Content publishing | Dev mode ✅ |
| `pages_show_list` | List user's pages | Dev mode ✅ |
| `pages_read_engagement` | Read page activity | Dev mode ✅ |
| `pages_manage_metadata` | Subscribe to webhooks | Dev mode ✅ |
| `pages_messaging` | Send messages via page | Dev mode ✅ |

---

## App Review (for production)

**What to prepare:**
1. Business Verification in Meta Business Portfolio
2. Privacy Policy page (HTTPS)
3. Screencast for each requested permission showing real usage
4. Test account for the reviewer
5. Use-case description per permission

**Timeline:** 2–7 days for review + Business Verification time.
**Dev mode limit:** up to 25 test accounts with a role in the app.

---

## Rate Limits

| Limit | Value |
|-------|-------|
| API calls | 200 / account / hour |
| Webhook timeout | Meta expects a response within 20 seconds |
| DM reply window | 24 hours after the user's message |
| Token lifetime | ~60 days (refresh on day 50) |

**Rate limiter implementation:**

```go
// Token bucket in Redis — 200 calls per account per hour
type RateLimiter struct {
    redis  *redis.Client
    limit  int
    window time.Duration
}

func (rl *RateLimiter) Allow(ctx context.Context, accountID string) bool {
    key := fmt.Sprintf("ratelimit:ig:%s", accountID)
    count, _ := rl.redis.Incr(ctx, key).Result()
    if count == 1 {
        rl.redis.Expire(ctx, key, rl.window)
    }
    return count <= int64(rl.limit)
}
```
