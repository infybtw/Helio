# Helio

Telegram group moderation bot written in Go. Receives updates via a webhook and provides owner-only moderation commands.

## Commands

Commands are used by replying to a message in a group. Admin commands can be used by the **group owner** and users granted rights with `!grant`.

### Admin commands (owner + granted users)

| Command | Description |
|---|---|
| `!delete` | Deletes the replied-to message |
| `!mute [duration]` | Mutes the author of the replied-to message and deletes it. Duration accepts bare minutes (`30`), Go syntax (`30m`, `1h30m`), or days (`1d`, `2d12h`). Default: 30 minutes |
| `!ban` | Permanently bans the author of the replied-to message and deletes it |

### Owner commands (group owner only)

| Command | Description |
|---|---|
| `!grant` | Grants command rights in the chat to the author of the replied-to message (stored in Postgres) |
| `!revoke` | Revokes command rights in the chat from the author of the replied-to message |

## Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and get a token.
2. Copy `.env_example` to `.env` and fill in the values:

   | Variable | Required | Description |
   |---|---|---|
   | `TELEGRAM_BOT_TOKEN` | yes | Bot token from @BotFather |
   | `WEBHOOK_URL` | yes | Public HTTPS URL Telegram will send updates to |
   | `WEBHOOK_SECRET` | no | Secret token for webhook verification (recommended) |
   | `WEBHOOK_PATH` | no | Local webhook endpoint path (default: `/webhook`) |
   | `PORT` | no | Port to listen on (default: `8080`) |
   | `ALLOWED_UPDATES` | no | Comma-separated update types (default: `message,channel_post,edited_message,edited_channel_post`) |
   | `LOG_LEVEL` | no | `debug`, `info`, `warn`, or `error` (default: `info`) |
   | `DATABASE_URL` | yes | Postgres connection URL |
   | `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | no | Postgres credentials for docker-compose (default: `heliobot`) |

3. Run the bot:

   ```sh
   go run ./apps/tg_bot
   ```

   On startup the bot registers the webhook with Telegram; on shutdown it removes it.

   Database migrations are applied with the separate `migrations` service:

   ```sh
   go run ./apps/migrations          # apply all pending migrations
   go run ./apps/migrations status   # show migration status
   ```

   Or run everything with Docker Compose (starts Postgres, applies migrations, then the bot):

   ```sh
    docker compose -f docker-compose.dev.yml up --build
    ```

4. Add the bot to your group and grant it admin rights (delete messages, ban users).

## Voice recognition

`apps/voice_recognizer` is a standalone FastAPI service backed by faster-whisper.
Docker Compose exposes it at `http://localhost:8000` and downloads the configured
Whisper model when the service starts.

| Endpoint | Description |
|---|---|
| `GET /health` | Returns `{"status":"ok"}` after the model is loaded |
| `POST /stt` | Transcribes `multipart/form-data` field `audio`; optional `language` query parameter |

```sh
curl -F 'audio=@voice.ogg' 'http://localhost:8000/stt?language=ru'
```

The response includes the complete `text`, detected `language`, confidence,
audio `duration`, and timestamped `segments`. Configure `WHISPER_MODEL`,
`WHISPER_DEVICE`, `WHISPER_COMPUTE_TYPE`, `WHISPER_CPU_THREADS`, and
`STT_MAX_UPLOAD_BYTES` in `.env`. `WHISPER_CPU_THREADS` limits CTranslate2
to that many CPU threads per transcription; use `0` for its default.

The Telegram bot transcribes new voice messages in groups and supergroups,
then replies to the source message with the recognized text. It stores each OGG
in JetStream Object Store, publishes a durable `stt.jobs` task, and receives the
result through the durable `stt.results` consumer. Docker Compose runs NATS with
JetStream enabled; use `NATS_URL=nats://localhost:4222` outside Compose. Disable
the bot's privacy mode in BotFather so it receives ordinary group voice messages.

## Run locally

Prerequisites: Go, Bun, Docker (for Postgres), and a public HTTPS tunnel. Telegram cannot send webhooks or complete the OIDC callback against `localhost`.

1. Copy and configure the local environment file:

   ```sh
   cp .env_example .env
   ```

   Set `DATABASE_URL` to `postgres://heliobot:heliobot@localhost:5432/heliobot`, `DASHBOARD_ORIGIN` to `http://localhost:3000`, and `COOKIE_SECURE` to `false`.

   Start a tunnel to the bot API on port `8080`, then use its public HTTPS URL in both values registered with BotFather:

   ```dotenv
   WEBHOOK_URL=https://your-public-url/webhook
   OIDC_REDIRECT_URI=https://your-public-url/api/auth/telegram/oidc/callback
   ```

   Set `SESSION_SECRET` to a strong random value, and add the Telegram Login Widget credentials (`OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET`) from BotFather.

2. Start Postgres and apply migrations from the repository root:

   ```sh
   docker compose -f docker-compose.dev.yml up -d postgres
   go run ./apps/migrations
   ```

3. Start the bot API in one terminal:

   ```sh
   go run ./apps/tg_bot
   ```

4. Start the dashboard in another terminal:

   ```sh
   cd apps/web
   bun install
    NUXT_PUBLIC_API_BASE_URL=https://yourdomain.com bun run dev
   ```

   Open [http://localhost:3000](http://localhost:3000). The dashboard connects to `https://rp1.infybtw.dev` in this local setup.

5. Stop the local database when finished:

   ```sh
   docker compose -f docker-compose.dev.yml down
   ```

## Project structure

```
apps/
├── migrations/                # database migration service (goose + pgx)
│   ├── main.go                # applies embedded goose migrations (up/down/status/...)
│   └── migrations/            # SQL migration files
├── voice_recognizer/          # FastAPI/faster-whisper speech-to-text service
└── tg_bot/
    ├── main.go                # entrypoint: config, database, webhook registration, HTTP server
    └── internal/
        ├── commands/          # command handlers (!delete, !mute, !ban, !grant, !revoke) and dispatch
        ├── config/            # environment configuration
        ├── handlers/          # webhook HTTP handler (secret token verification)
        ├── logger/            # colorized slog handler
        ├── store/             # Postgres access layer (pgx)
        └── telegram/          # thin Telegram Bot API client and types
```
