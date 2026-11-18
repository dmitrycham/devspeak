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

The words REST API lives in `go-api/`. It reads settings from the root `.env`.

### Run

```bash
docker compose up -d --wait db
docker compose run --rm migrate up
cd go-api && go run ./cmd/api      # listens on HTTP_ADDR (default :8080)
```

### Endpoints

| Method | Path          | Success | Errors                                   |
|--------|---------------|---------|------------------------------------------|
| POST   | `/words`      | 201     | 400 invalid body, 409 duplicate word     |
| GET    | `/words`      | 200     |                                          |
| GET    | `/words/{id}` | 200     | 400 bad id, 404 not found                |
| DELETE | `/words/{id}` | 204     | 400 bad id, 404 not found                |

```bash
curl -i -X POST localhost:8080/words \
  -H 'Content-Type: application/json' \
  -d '{"word":"goroutine","translation":"горутина","example":"Start a goroutine."}'
curl -i localhost:8080/words
curl -i localhost:8080/words/1
curl -i -X DELETE localhost:8080/words/1
```

### Test and lint

```bash
cd go-api && go test -race ./... && golangci-lint run
```
