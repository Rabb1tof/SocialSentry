# Развёртывание на EasyPanel (production)

Прод тянет готовые образы из **GHCR** (их собирает `release.yml` при пуше тега
`v*.*.*`) — на сервере ничего не билдится. Стек описан в
[`docker-compose.prod.yml`](../docker-compose.prod.yml).

Образы:
`ghcr.io/rabb1tof/socialsentry/{api,worker,migrate,frontend}`.

## Архитектура в проде (важно)

Один домен на всё. Фронтенд (nginx) раздаёт SPA и проксирует на внутренний `api`:

```
                ┌─────────────────────────────────────────┐
Меня/браузер →  │ frontend (nginx)  :80                     │
  https://дом   │   /            → SPA (index.html)         │
                │   /api/        → api:8080                 │
                │   /webhooks    → api:8080  (Meta)         │
                │   /ws          → api:8080  (WebSocket)    │
                └───────────────┬───────────────────────────┘
                                │ внутренняя сеть
              ┌─────────┬───────┴───────┬──────────┐
            api:8080  worker        postgres:5432  redis:6379
```

Почему один домен: фронт ходит в API по относительному пути `/api/v1`, а refresh-токен
лежит в httpOnly-куке с `SameSite=Strict`. Разнести фронт и API по разным доменам без
правки кода нельзя — кука и CORS сломаются. Поэтому Meta webhook и OAuth-redirect тоже
идут через тот же домен (`/webhooks/instagram`, `/api/v1/accounts/instagram/callback`).

---

## Шаг 0. Сначала выпусти релиз

EasyPanel тянет образы по версии, поэтому они должны существовать в GHCR:

```bash
git tag v1.0.0
git push origin v1.0.0      # release.yml соберёт и запушит 4 образа + создаст Release
```

Проверь во вкладке **Packages** репозитория, что появились `api`, `worker`,
`migrate`, `frontend` с тегом `1.0.0` и `latest`.

### Доступ к приватным образам

По умолчанию пакеты GHCR приватные. Выбери одно:

- **Проще всего:** сделать пакеты публичными — у каждого пакета *Package settings →
  Change visibility → Public*. Тогда EasyPanel тянет их без авторизации.
- **Для закрытого кода:** создай GitHub PAT со scope `read:packages` и добавь его в
  EasyPanel как Docker-registry креды (см. шаг 2), реестр `ghcr.io`, логин — твой
  GitHub-username.

---

## Шаг 1. Проект и Compose-сервис

1. В EasyPanel создай **Project** (например `socialsentry`).
2. Внутри проекта: **+ Service → Compose**.
3. Источник compose — любой из:
   - **Git**: репозиторий `Rabb1tof/SocialSentry`, ветка `main`, файл
     `docker-compose.prod.yml`; либо
   - **Inline**: вставь содержимое [`docker-compose.prod.yml`](../docker-compose.prod.yml).

## Шаг 2. Переменные окружения

В поле **Environment** Compose-сервиса задай (это значения для интерполяции
`${...}` в compose). Минимум:

```env
VERSION=1.0.0
DB_PASSWORD=<надёжный-пароль>
JWT_SECRET=<openssl rand -hex 32>
ENCRYPTION_KEY=<openssl rand -hex 32>
META_APP_ID=1388425723092623
META_APP_SECRET=<секрет>
META_WEBHOOK_VERIFY_TOKEN=<любая-строка>
META_CALLBACK_URL=https://<твой-домен>/webhooks/instagram
VK_API_VERSION=5.199
LOG_LEVEL=info
```

Полный список с пояснениями — в [`.env.prod.example`](../.env.prod.example).
`DB_NAME`/`DB_USER` можно не задавать — по умолчанию `socialsentry`.

> Если образы приватные и ты не делал их публичными — добавь GHCR-креды:
> *Project → Settings* (или раздел Docker Registry) → registry `ghcr.io`,
> username = GitHub-логин, password = PAT с `read:packages`.

## Шаг 3. Домен и SSL

1. Открой Compose-сервис → вкладка **Domains**.
2. Добавь домен и привяжи его к сервису **frontend**, контейнерный порт **80**.
3. EasyPanel (Traefik) сам выпустит Let's Encrypt-сертификат.

Порт `8088`, проброшенный в compose, нужен только для «голого» Docker — в EasyPanel
маршрутизацию делает домен, его можно игнорировать.

## Шаг 4. Деплой

Нажми **Deploy**. Порядок старта:

1. `postgres`, `redis` поднимаются и проходят healthcheck.
2. `migrate` одноразово прогоняет `goose up` (идемпотентно) и завершается.
3. `api`, `worker` стартуют только после успешного `migrate`.
4. `frontend` проксирует на `api`.

## Шаг 5. Настрой Meta

В Meta Developer Console webhook и OAuth-redirect должны указывать на твой домен:

- Callback URL: `https://<твой-домен>/webhooks/instagram`
- Verify Token: значение `META_WEBHOOK_VERIFY_TOKEN`
- Подписки: `messages`, `comments`
- OAuth redirect URI: `https://<твой-домен>/api/v1/accounts/instagram/callback`

(VK работает через Bots Long Poll — входящий вебхук не нужен.)

---

## Обновление до новой версии

1. Выпусти новый тег (`git tag v1.1.0 && git push origin v1.1.0`) — дождись, пока
   `release.yml` опубликует образы.
2. В EasyPanel поменяй `VERSION=1.1.0` и нажми **Deploy** (или **Redeploy** /
   **Pull**). `migrate` снова прогонит новые миграции перед стартом api/worker.

## Заметки

- Сервисы внутри проекта/compose видят друг друга по имени: `api`, `postgres`,
  `redis`. Поэтому `API_UPSTREAM=api:8080` и хост БД `postgres` работают как есть.
  Если твоя версия EasyPanel использует префиксные хостнеймы — посмотри точное имя
  в Credentials БД и поправь `API_UPSTREAM` / DSN.
- Бэкапы: Postgres и Redis живут в volume'ах внутри стека. Бэкап настраиваешь сам
  (например `pg_dump` по cron). Альтернатива — вынести БД/Redis в управляемые
  сервисы EasyPanel и убрать их из compose, заменив `DATABASE_URL`/`REDIS_URL`.
- Очереди Asynq (IG) можно смотреть, временно подняв `hibiken/asynqmon` рядом и
  указав ему `--redis-addr=redis:6379` (см. [`deployment.md`](./deployment.md)).

---

## Без EasyPanel — голый Docker-хост

```bash
git clone https://github.com/Rabb1tof/SocialSentry.git && cd SocialSentry
cp .env.prod.example .env.prod      # заполни секреты, задай VERSION
docker login ghcr.io                # если образы приватные
docker compose --env-file .env.prod -f docker-compose.prod.yml pull
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d
```

Фронтенд будет на `http://<host>:8088`. Для HTTPS поставь перед ним обратный
прокси (Caddy/Traefik/nginx) с сертификатом и проксируй на порт 8088.
