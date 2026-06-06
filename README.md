# SocialSentry

> SaaS-платформа автоответов на комментарии и личные сообщения в **Instagram** и **VK**.

Мультиаккаунтность · Гибкие триггеры · Система подписок · Роль администратора.

---

## Документация

Полная документация находится в [`docs/`](./docs/):

| Файл | Описание |
|------|---------|
| [`docs/README.md`](./docs/README.md) | Обзор и навигация по документам |
| [`docs/plan.md`](./docs/plan.md) | План реализации по фазам |
| [`docs/architecture.md`](./docs/architecture.md) | Стек, диаграммы, структура файлов |
| [`docs/instagram-api.md`](./docs/instagram-api.md) | Instagram API (проверено на практике) |
| [`docs/vk-api.md`](./docs/vk-api.md) | VK API |
| [`docs/database.md`](./docs/database.md) | Схема БД и миграции |
| [`docs/deployment.md`](./docs/deployment.md) | Деплой, Docker, CI/CD |
| [`docs/easypanel.md`](./docs/easypanel.md) | Прод-деплой на EasyPanel (образы из GHCR) |
| [`docs/agents.md`](./docs/agents.md) | Правила и контекст для AI агентов |

Файл [`CLAUDE.md`](./CLAUDE.md) в корне — копия `docs/agents.md` для Claude Code (с поправленными путями к документам).

---

## Требования

- Go 1.22+
- Node.js 20+
- Docker + Docker Compose
- ngrok (для тестирования Meta Webhook локально)

## Быстрый старт

```bash
cp .env.example .env
# Заполнить JWT_SECRET, ENCRYPTION_KEY, META_APP_SECRET, ...

docker compose up -d postgres redis
cd backend && goose -dir internal/db/migrations postgres "$DATABASE_URL" up

docker compose up api worker frontend
```

Полная инструкция: [`docs/README.md`](./docs/README.md) → раздел «Быстрый старт».

## Прод

Релизы публикуются как образы в GHCR при пуше тега `v*.*.*` (workflow
[`release.yml`](./.github/workflows/release.yml)). Прод-стек
[`docker-compose.prod.yml`](./docker-compose.prod.yml) их **тянет**, а не билдит.
Деплой на EasyPanel — [`docs/easypanel.md`](./docs/easypanel.md).

```bash
git tag v1.0.0 && git push origin v1.0.0   # собрать и опубликовать образы
```
