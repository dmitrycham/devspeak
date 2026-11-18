package cards

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// pgUniqueViolation is the PostgreSQL error code for a unique constraint error.
const pgUniqueViolation = "23505"

// cardColumns is the column list for every SELECT/RETURNING.
// The order must match scanCard.
const cardColumns = `id, phrase, translation, example, due_at, interval_days, ease, repetitions, created_at`

// PostgresRepository stores cards in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a repository that uses the given connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// scanCard reads one row in the cardColumns order.
// Both pgx.Row and pgx.Rows have a Scan method, so it works for both.
func scanCard(row interface{ Scan(dest ...any) error }) (Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.Phrase, &c.Translation, &c.Example,
		&c.DueAt, &c.IntervalDays, &c.Ease, &c.Repetitions, &c.CreatedAt)
	return c, err
}

// Create inserts a card. It returns ErrDuplicate if the phrase already exists
// (case-insensitive, see the cards_phrase_lower_idx index).
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (Card, error) {
	query := `INSERT INTO cards (phrase, translation, example)
		VALUES ($1, $2, $3)
		RETURNING ` + cardColumns

	c, err := scanCard(r.pool.QueryRow(ctx, query, in.Phrase, in.Translation, in.Example))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return Card{}, ErrDuplicate
		}
		return Card{}, fmt.Errorf("insert card: %w", err)
	}
	return c, nil
}

// List returns all cards, oldest first.
func (r *PostgresRepository) List(ctx context.Context) ([]Card, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cardColumns+` FROM cards ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	cards := []Card{} // not nil: an empty list is encoded as [] in JSON, not null
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	return cards, nil
}

// Get returns one card by id, or ErrNotFound.
func (r *PostgresRepository) Get(ctx context.Context, id int64) (Card, error) {
	c, err := scanCard(r.pool.QueryRow(ctx, `SELECT `+cardColumns+` FROM cards WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, fmt.Errorf("get card %d: %w", id, err)
	}
	return c, nil
}

// Delete removes one card by id, or returns ErrNotFound.
func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cards WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// NextDue returns the card that has waited the longest (due_at <= now),
// or ErrNotFound if no card is due.
func (r *PostgresRepository) NextDue(ctx context.Context, now time.Time) (Card, error) {
	query := `SELECT ` + cardColumns + ` FROM cards
		WHERE due_at <= $1
		ORDER BY due_at, id
		LIMIT 1`

	c, err := scanCard(r.pool.QueryRow(ctx, query, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, fmt.Errorf("next due card: %w", err)
	}
	return c, nil
}

// UpdateSchedule saves a new spaced repetition state and due time.
// It returns the updated card, or ErrNotFound.
func (r *PostgresRepository) UpdateSchedule(ctx context.Context, id int64, st srs.State, dueAt time.Time) (Card, error) {
	query := `UPDATE cards
		SET repetitions = $2, interval_days = $3, ease = $4, due_at = $5
		WHERE id = $1
		RETURNING ` + cardColumns

	c, err := scanCard(r.pool.QueryRow(ctx, query, id, st.Repetitions, st.IntervalDays, st.Ease, dueAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, fmt.Errorf("update card %d schedule: %w", id, err)
	}
	return c, nil
}
