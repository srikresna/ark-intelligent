# ARK Intelligent

Institutional-grade macro intelligence Telegram bot for forex traders, delivering COT positioning, economic calendar events, FRED macro data, and AI-powered analysis.

## Features

- **COT Analysis** -- Weekly CFTC Commitment of Traders positioning with net change, percentile ranking, and signal detection
- **Economic Calendar** -- Real-time high-impact event tracking via MQL5 Economic Calendar with revision detection and surprise scoring
- **FRED Macro Intelligence** -- Federal Reserve economic data monitoring with regime change alerts (yield curve, labor market, inflation)
- **AI Interpretation** -- Gemini-powered narrative summaries, weekly outlooks, and confluence scoring (graceful template fallback when offline)
- **Signal Detection** -- Multi-factor conviction scoring combining COT positioning, macro surprises, and FRED regime data

## Architecture

Hexagonal (ports & adapters) architecture in Go 1.22:

- **Storage**: BadgerDB (embedded key-value store, zero external dependencies)
- **Messaging**: Telegram Bot API (long-polling)
- **AI**: Google Gemini with caching layer and rate limiting
- **Data Sources**: CFTC Socrata API, MQL5 Economic Calendar, FRED API
- **Scheduling**: Built-in background job scheduler with graceful shutdown

## Quick Start

1. Copy the environment template and fill in your values:
   ```bash
   cp .env.example .env
   ```

2. Set at minimum `BOT_TOKEN` and `CHAT_ID`. Optionally add `GEMINI_API_KEY` for AI features and `FRED_API_KEY` for macro data (free at https://fred.stlouisfed.org/docs/api/api_key.html).

3. Run with Docker Compose:
   ```bash
   docker-compose up -d
   ```

   Or build and run directly:
   ```bash
   go build -o ark-intelligent ./cmd/bot
   ./ark-intelligent
   ```

## Telegram Commands

| Command      | Description                                      |
|--------------|--------------------------------------------------|
| `/cot`       | COT positioning summary (all currencies or one)  |
| `/calendar`  | Upcoming high-impact economic events              |
| `/outlook`   | AI-generated weekly macro outlook                 |
| `/bias`      | Directional bias from COT positioning analysis    |
| `/rank`      | Currency strength ranking from COT + macro data   |
| `/macro`     | FRED macro dashboard (yields, labor, inflation)   |
| `/prefs`     | Configure personal alert preferences              |
| `/help`      | List all available commands                        |

## Data Sources

| Source                | Data                          | Update Frequency |
|-----------------------|-------------------------------|------------------|
| CFTC Socrata API      | Commitment of Traders reports | Weekly (Friday)  |
| MQL5 Economic Calendar| Economic events and actuals   | Real-time        |
| FRED (St. Louis Fed)  | Macro indicators              | Varies by series |

## Configuration

| Variable             | Required | Default                        | Description                        |
|----------------------|----------|--------------------------------|------------------------------------|
| `BOT_TOKEN`          | Yes      | --                             | Telegram bot token from BotFather  |
| `CHAT_ID`            | Yes      | --                             | Default Telegram chat ID           |
| `GEMINI_API_KEY`     | No       | --                             | Google Gemini API key              |
| `GEMINI_MODEL`       | No       | `gemini-3.1-flash-lite-preview`| Gemini model name                  |
| `DATA_DIR`           | No       | `/app/data`                    | BadgerDB storage directory         |
| `COT_FETCH_INTERVAL` | No       | `6h`                           | How often to fetch COT data        |
| `COT_HISTORY_WEEKS`  | No       | `52`                           | Weeks of COT history to retain     |
| `AI_CACHE_TTL`       | No       | `1h`                           | AI response cache duration         |
| `AI_MAX_RPM`         | No       | `15`                           | AI requests per minute limit       |
| `LOG_LEVEL`          | No       | `info`                         | Logging verbosity                  |

## License

All rights reserved. See LICENSE file for details.
