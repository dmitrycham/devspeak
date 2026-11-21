# DevSpeak

Smart vocabulary and interview trainer: Go API + Python AI service.

## Database

PostgreSQL 16 with pgvector runs in Docker. Migrations live in
`go-api/migrations` and run with [golang-migrate](https://github.com/golang-migrate/migrate)
inside Docker, so you don't need to install it.

### First start

```bash
cp .env.example .env              # then edit the values in .env
docker compose up -d db           # start PostgreSQL in the background
docker compose ps                 # wait until db is "healthy"
docker compose run --rm migrate up
```

### Migrations

```bash
docker compose run --rm migrate up        # apply all new migrations
docker compose run --rm migrate down 1    # roll back the last migration
docker compose run --rm migrate version   # show the current version
```

Create a new migration (it adds `NNNNNN_<name>.up.sql` and `.down.sql`):

```bash
docker run --rm -v "$PWD/go-api/migrations:/migrations" migrate/migrate:v4.20.1 \
  create -ext sql -dir /migrations -seq <name>
```

### Connect with psql

```bash
docker compose exec db sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

### Stop and reset

```bash
docker compose down        # stop; data stays in the pgdata volume
docker compose down -v     # stop and DELETE all data
```

## Go API

The cards REST API lives in `go-api/`. It reads settings from the root `.env`.

### Run

```bash
docker compose up -d --wait db
docker compose run --rm migrate up
cd go-api && go run ./cmd/api      # listens on HTTP_ADDR (default :8080)
```

### Endpoints

| Method | Path            | Success                 | Errors                                       |
|--------|-----------------|-------------------------|----------------------------------------------|
| POST   | `/cards`        | 201                     | 400 invalid body, 409 duplicate phrase       |
| GET    | `/cards`        | 200                     |                                              |
| GET    | `/cards/{id}`   | 200                     | 400 bad id, 404 not found                    |
| DELETE | `/cards/{id}`   | 204                     | 400 bad id, 404 not found                    |
| POST   | `/cards/{id}/enrich` | 200 updated card   | 400 bad id, 404 not found, 502 AI service failed, 504 AI timeout |
| GET    | `/reviews/next` | 200 card, 204 none due  |                                              |
| POST   | `/reviews/{id}` | 200 updated card        | 400 bad id / body / grade, 404 not found     |

```bash
curl -i -X POST localhost:8080/cards \
  -H 'Content-Type: application/json' \
  -d '{"phrase":"goroutine","translation":"горутина","example":"Start a goroutine."}'
curl -i localhost:8080/cards
curl -i localhost:8080/reviews/next
curl -i -X POST localhost:8080/reviews/1 \
  -H 'Content-Type: application/json' -d '{"grade":"knew"}'      # or "forgot"
curl -i -X POST localhost:8080/cards/1/enrich                    # needs ai-service
```

### Enrich with the AI service

`POST /cards/{id}/enrich` sends the phrase to `ai-service` (`POST /enrich`) and saves
`it_context`. The example is filled only if the card has none: the user's example wins.
A card that already has `it_context` is returned as is, without an AI call,
so repeated calls are cheap and add no data. If two requests enrich the same card
at the same time, the first saved answer wins and both get the same card.

| Variable         | Default                 | Meaning                                  |
|------------------|-------------------------|------------------------------------------|
| `AI_SERVICE_URL` | `http://localhost:8000` | base URL of `ai-service`                 |
| `AI_TIMEOUT`     | `30s`                   | limit for one AI call (Go duration)      |

The server's write timeout is `AI_TIMEOUT + 10s`, so a slow AI answer still reaches the client.

### Spaced repetition (SM-2, two grades)

New cards are due now. `internal/srs` computes the next review:

- **knew**: interval 1 day → 3 days → interval × ease (rounded). Ease does not change.
- **forgot**: the card starts again and is due now; ease − 0.2 (minimum 1.3).

## Telegram bot

The bot runs on your machine with long polling: no public server needed.
It uses the same service layer and database as the API.

### Create the bot

1. In Telegram, open [@BotFather](https://t.me/BotFather) and send `/newbot`.
2. Choose a name and a username (must end with `bot`). BotFather sends you the token.
3. Open [@userinfobot](https://t.me/userinfobot) to get your numeric Telegram user id.
4. Put both values in `.env` (never commit it):

```bash
TELEGRAM_BOT_TOKEN=<token from BotFather>
TELEGRAM_ALLOWED_USER_ID=<your numeric user id>
```

Only this user id can use the bot. Messages from other users are ignored.

### Run

```bash
docker compose up -d --wait db
docker compose run --rm migrate up
cd go-api && go run ./cmd/bot      # stop with Ctrl+C
```

### Commands

- `/add It stops accepting new requests — перестаёт принимать новые запросы`
  saves a card (the separator is `—` or ` - `). You can send several `/add` lines
  in one message: each line is one card, and phrases that already exist are skipped.
- `/review` shows the Russian side. Say the English chunk aloud, press **Show**,
  then press **✅ Knew** or **❌ Forgot**. The next due card comes right after.
  When nothing is due, the bot says "All done for today".

### Test and lint

```bash
cd go-api && go test -race ./... && golangci-lint run
```

Repository tests on a real PostgreSQL (needs Docker running; testcontainers
starts and removes the container). `-count=1` turns off the test cache, so a
change in `migrations/*.sql` always runs the tests again:

```bash
cd go-api && go test -count=1 -tags=integration ./internal/cards/...
```
