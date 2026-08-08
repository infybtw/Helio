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

## Project structure

```
apps/
├── migrations/                # database migration service (goose + pgx)
│   ├── main.go                # applies embedded goose migrations (up/down/status/...)
│   └── migrations/            # SQL migration files
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
