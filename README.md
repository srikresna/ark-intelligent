# FF Economic Calendar Telegram Bot

Fast, zero-dependency Go bot that sends Forex Factory economic calendar data to Telegram.
No AI, no bullshit — just raw data from ForexFactory with **per-user personalized alerts**.

## Features

### Calendar (single menu)
- **Today** — Today's economic events (WIB timezone)
- **This Week** — Full week calendar grouped by day
- **High Impact** — High impact events only
- **Next Events** — Next 10 upcoming events with countdown
- **Refresh** — Force data refresh from ForexFactory

### Per-User Alert Preferences
- **Alert Timing** — Choose when to be alerted: 60m, 30m, 15m, 5m, 1m before events (multi-select)
- **Impact Filter** — Choose which impact levels to receive: High, Medium, Low (multi-select)
- **Toggle Alerts** — Enable/disable alerts entirely
- **Status** — View your current settings and bot status
- Preferences persist across bot restarts (JSON file storage)
- Each user in a group can have different settings

### Auto Alerts
- **Pre-event alerts** — Sent at your chosen minutes before each event
- **Result alerts** — Sent when actual data is released with BEAT/MISS/IN LINE detection
- **Smart refresh** — 2min polling near events, 5min when idle

## Menu Structure

```
/start
├── 📅 Calendar
│   ├── 📆 Today
│   ├── 📋 This Week
│   ├── 🔴 High Impact
│   ├── ⏭ Next Events
│   ├── 🔄 Refresh
│   └── ← Back
└── ⚙️ Settings
    ├── ⏰ Alert Timing (toggle 60m/30m/15m/5m/1m)
    ├── 📊 Impact Filter (toggle High/Medium/Low)
    ├── 🔔 Toggle Alerts (on/off)
    ├── 📋 Status
    └── ← Back
```

## Data Source

Forex Factory official JSON: `https://nfs.faireconomy.media/ff_calendar_thisweek.json`
- Refreshed every 2-5 minutes automatically
- Contains: title, country, impact, forecast, previous, actual, datetime
- No scraping, no Cloudflare issues
- **Note**: Only current week data is available (no historical, no next week)

## Quick Start

### Option 1: Docker (recommended)

```bash
cp .env.example .env
# Edit .env with your BOT_TOKEN and CHAT_ID
docker compose up -d
docker compose logs -f
```

### Option 2: Run directly (requires Go 1.22+)

```bash
export BOT_TOKEN="your-telegram-bot-token"
export CHAT_ID="your-chat-id"
go build -o ffbot main.go
./ffbot
```

### Option 3: Docker manual build

```bash
docker build -t ffbot .
docker run -d --name ffbot \
  -e BOT_TOKEN="your-token" \
  -e CHAT_ID="your-chat-id" \
  -v ffbot-data:/app/data \
  --restart always \
  ffbot
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `BOT_TOKEN` | Yes | Telegram Bot API token from @BotFather |
| `CHAT_ID` | No | Default chat/group ID for legacy alerts. Without this, alerts only go to users who have interacted with the bot |
| `PREFS_FILE` | No | Path to preferences JSON file (default: `/app/data/prefs.json`) |

## Getting Your CHAT_ID

1. Start the bot without CHAT_ID
2. Send `/chatid` to the bot in your target chat/group
3. Copy the ID and set it as CHAT_ID env var
4. Restart the bot

## Default Alert Settings

New users get these defaults (can be changed via Settings):
- **Timing**: 30m, 15m, 5m before events
- **Impact**: High only
- **Alerts**: ON

## Architecture

- **Zero external dependencies** — only Go stdlib
- **3 goroutines**: command polling, data refresher, alert scheduler
- **Per-user preferences**: JSON file persistence (`prefs.json`)
- **Runs on any VPS**: $3-5/month is more than enough

## License

MIT — do whatever you want
