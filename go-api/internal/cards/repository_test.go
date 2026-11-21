//go:build integration

// Integration tests for PostgresRepository against a real PostgreSQL in Docker.
// Run them with: go test -count=1 -tags=integration ./internal/cards/...
package cards

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// Same image as the db service in docker-compose.yml, so tests see the same PostgreSQL.
const testImage = "pgvector/pgvector:0.8.7-pg16"

// testPool is shared by all tests in this package. One container for the whole
// package keeps the run fast; tests stay independent through newRepo.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	// os.Exit skips deferred calls, so the work with defers lives in run.
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	// The postgres image runs *.sql files from /docker-entrypoint-initdb.d in name
	// order before it accepts connections, so the up migrations apply in sequence.
	scripts, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(scripts) == 0 {
		log.Printf("find migrations: %v (found %d files)", err, len(scripts))
		return 1
	}

	ctr, err := postgres.Run(ctx, testImage,
		postgres.WithDatabase("devspeak_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts(scripts...),
		postgres.BasicWaitStrategies(),
	)
	// Run can return a container together with an error, so clean up before the check.
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			log.Printf("terminate container: %v", err)
		}
	}()
	if err != nil {
		log.Printf("start postgres: %v", err)
		return 1
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("connection string: %v", err)
		return 1
	}

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		log.Printf("connect: %v", err)
		return 1
	}
	defer testPool.Close()

	return m.Run()
}

// newRepo empties the table and returns a repository on the shared pool.
// A rollback-per-test is not possible: the repository takes a pool, not a transaction.
// RESTART IDENTITY makes ids start from 1 again, so tests can rely on them.
func newRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	if _, err := testPool.Exec(t.Context(), `TRUNCATE cards RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate cards: %v", err)
	}
	return NewPostgresRepository(testPool)
}

func mustCreate(t *testing.T, repo *PostgresRepository, phrase string) Card {
	t.Helper()
	c, err := repo.Create(t.Context(), CreateInput{Phrase: phrase, Translation: "перевод", Example: "An example."})
	if err != nil {
		t.Fatalf("create %q: %v", phrase, err)
	}
	return c
}

// assertSameCard compares cards field by field: time.Time must be compared with Equal, not ==.
func assertSameCard(t *testing.T, want, got Card) {
	t.Helper()
	if got.ID != want.ID || got.Phrase != want.Phrase || got.Translation != want.Translation ||
		got.Example != want.Example || got.IntervalDays != want.IntervalDays ||
		got.Ease != want.Ease || got.Repetitions != want.Repetitions ||
		!got.DueAt.Equal(want.DueAt) || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("card mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestRepository_Create(t *testing.T) {
	repo := newRepo(t)
	before := time.Now()

	c, err := repo.Create(t.Context(), CreateInput{Phrase: "roll out", Translation: "выкатить", Example: "We roll out on Friday."})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if c.ID != 1 || c.Phrase != "roll out" || c.Translation != "выкатить" || c.Example != "We roll out on Friday." {
		t.Errorf("unexpected card fields: %+v", c)
	}
	// Defaults come from the migrations, not from Go code.
	if c.Ease != 2.5 || c.IntervalDays != 0 || c.Repetitions != 0 {
		t.Errorf("unexpected SRS defaults: ease=%v interval=%d reps=%d", c.Ease, c.IntervalDays, c.Repetitions)
	}
	// A new card is due now. The tolerance covers clock skew between host and container.
	if d := c.DueAt.Sub(before); d < -time.Minute || d > time.Minute {
		t.Errorf("due_at %v is not close to now %v", c.DueAt, before)
	}
}

func TestRepository_Create_DuplicateIsCaseInsensitive(t *testing.T) {
	repo := newRepo(t)
	mustCreate(t, repo, "roll out")

	for _, phrase := range []string{"roll out", "Roll Out", "ROLL OUT"} {
		t.Run(phrase, func(t *testing.T) {
			_, err := repo.Create(t.Context(), CreateInput{Phrase: phrase, Translation: "x"})
			if !errors.Is(err, ErrDuplicate) {
				t.Errorf("Create(%q) error = %v, want ErrDuplicate", phrase, err)
			}
		})
	}

	list, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("got %d cards, want 1", len(list))
	}
}

func TestRepository_Get(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, "deploy")

	got, err := repo.Get(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertSameCard(t, created, got)

	if _, err := repo.Get(t.Context(), created.ID+1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestRepository_List(t *testing.T) {
	repo := newRepo(t)

	empty, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Not nil: the handler encodes an empty list as [] in JSON.
	if empty == nil || len(empty) != 0 {
		t.Errorf("empty List = %#v, want empty non-nil slice", empty)
	}

	// "b" before "a": the order must follow id, not the phrase.
	b := mustCreate(t, repo, "b")
	a := mustCreate(t, repo, "a")

	list, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d cards, want 2", len(list))
	}
	assertSameCard(t, b, list[0])
	assertSameCard(t, a, list[1])
}

func TestRepository_Delete(t *testing.T) {
	repo := newRepo(t)
	c := mustCreate(t, repo, "rollback")

	if err := repo.Delete(t.Context(), c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(t.Context(), c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(t.Context(), c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete error = %v, want ErrNotFound", err)
	}
}

func TestRepository_UpdateSchedule(t *testing.T) {
	repo := newRepo(t)
	c := mustCreate(t, repo, "hotfix")

	// Values differ from the defaults (0, 0, 2.5), so a column that is not saved shows up.
	st := srs.State{Repetitions: 3, IntervalDays: 6, Ease: 1.7}
	dueAt := time.Now().Add(6 * 24 * time.Hour).Truncate(time.Microsecond)

	if _, err := repo.UpdateSchedule(t.Context(), c.ID, st, dueAt); err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}

	// Read back with Get: comparing with the RETURNING row alone would not catch
	// a column missing from the UPDATE, because both would hold the old value.
	got, err := repo.Get(t.Context(), c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := c
	want.Repetitions, want.IntervalDays, want.Ease, want.DueAt = st.Repetitions, st.IntervalDays, st.Ease, dueAt
	assertSameCard(t, want, got)

	if _, err := repo.UpdateSchedule(t.Context(), c.ID+1, st, dueAt); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateSchedule(missing) error = %v, want ErrNotFound", err)
	}
}

func TestRepository_NextDue(t *testing.T) {
	tests := []struct {
		name string
		// dueOffsets creates one card per offset, due at now+offset.
		dueOffsets []time.Duration
		wantIndex  int // index in dueOffsets of the expected card
		wantErr    error
	}{
		{name: "no cards", wantErr: ErrNotFound},
		{name: "only future cards", dueOffsets: []time.Duration{time.Hour}, wantErr: ErrNotFound},
		{name: "due exactly now", dueOffsets: []time.Duration{0}, wantIndex: 0},
		{name: "longest waiting card wins", dueOffsets: []time.Duration{-time.Hour, -2 * time.Hour, time.Hour}, wantIndex: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo(t)
			// PostgreSQL stores microseconds; truncating keeps "due exactly now" exact.
			now := time.Now().Truncate(time.Microsecond)

			var cards []Card
			for i, off := range tt.dueOffsets {
				c := mustCreate(t, repo, fmt.Sprintf("phrase %d", i))
				c, err := repo.UpdateSchedule(t.Context(), c.ID, c.State(), now.Add(off))
				if err != nil {
					t.Fatalf("UpdateSchedule: %v", err)
				}
				cards = append(cards, c)
			}

			got, err := repo.NextDue(t.Context(), now)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NextDue error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NextDue: %v", err)
			}
			assertSameCard(t, cards[tt.wantIndex], got)
		})
	}
}
