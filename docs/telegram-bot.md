# Telegram bot service

HireRadar keeps Telegram account linking, match queries, notification preferences, application requests, and feedback in the backend notification/application packages. `cmd/telegram-bot` owns Telegram update transport and webhook lifecycle. The backend API retains authenticated `/api/v1/telegram/*` routes for the web app and does not register a second Telegram webhook.

The bot uses Telegram webhook mode only. It sets the webhook during startup, verifies `X-Telegram-Bot-Api-Secret-Token`, caps update bodies at 64 KiB, and provides `/health` and PostgreSQL-backed `/ready` endpoints. `SIGINT` and `SIGTERM` trigger graceful HTTP shutdown. Telegram delivery remains in the existing backend notification worker; a Telegram user blocked by the bot is marked disabled and outstanding notifications are cancelled. HTTP 429 `retry_after` values are honored, bounded to one hour, and other transient failures use bounded exponential retry.

## Local Docker Compose

The normal `docker compose up` does not start the bot, so local development works without Telegram credentials. To start it, configure the values below in `.env`, set `TELEGRAM_WEBHOOK_URL` to a public HTTPS URL ending in `/telegram/webhook` (for example a trusted tunnel to port 8090), then run:

```sh
docker compose --profile telegram up -d --build telegram-bot
```

The local service listens on `${TELEGRAM_PORT:-8090}`. Health checks are available at `/health` and `/ready`.

## Railway deployment

Create one Railway service named `telegram-bot` using the existing PostgreSQL service. Set the service Dockerfile to `apps/backend/Dockerfile`, repository root as the build context, and the start command to `/app/telegram-bot`. Railway supplies `PORT`; the webhook URL must be the resulting public HTTPS domain plus `/telegram/webhook`. Configure a health check path `/ready`. The existing `backend` API continues to own API routes and backend workers; only the Telegram bot service registers the Telegram webhook.

Required Railway variables:

- `DATABASE_URL` — the same HireRadar PostgreSQL database used by the backend.
- `TELEGRAM_BOT_TOKEN` — BotFather token; store as a Railway secret.
- `TELEGRAM_BOT_USERNAME` — public bot username, without `@`.
- `TELEGRAM_WEBHOOK_URL` — public HTTPS bot service URL ending in `/telegram/webhook`.
- `TELEGRAM_WEBHOOK_SECRET` — random 1–256 character Telegram webhook secret using letters, digits, `_`, and `-`.
- `PORT` — supplied by Railway; local default is `8090`.

The backend `worker` still needs `TELEGRAM_BOT_TOKEN` to send notifications. It must not receive or register a webhook. Do not configure a second webhook URL in API transport or point Telegram updates at the backend API.
