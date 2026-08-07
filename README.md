# HelioBot

Telegram group moderation bot written in Go. Receives updates via a webhook and provides owner-only moderation commands.

## Commands

Commands are used by replying to a message in a group. Admin commands can only be used by the **group owner**.

### Admin commands (group owner only)

| Command | Description |
|---|---|
| `!delete` | Deletes the replied-to message |
| `!mute [duration]` | Mutes the author of the replied-to message and deletes it. Duration accepts bare minutes (`30`), Go syntax (`30m`, `1h30m`), or days (`1d`, `2d12h`). Default: 30 minutes |
| `!ban` | Permanently bans the author of the replied-to message and deletes it |

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

3. Run the bot:

   ```sh
   go run ./apps/tg_bot
   ```

   On startup the bot registers the webhook with Telegram; on shutdown it removes it.

4. Add the bot to your group and grant it admin rights (delete messages, ban users).

## Project structure

```
apps/tg_bot/
├── main.go                    # entrypoint: config, webhook registration, HTTP server
└── internal/
    ├── commands/              # command handlers (!delete, !mute, !ban) and dispatch
    ├── config/                # environment configuration
    ├── handlers/              # webhook HTTP handler (secret token verification)
    ├── logger/                # colorized slog handler
    └── telegram/              # thin Telegram Bot API client and types
```
