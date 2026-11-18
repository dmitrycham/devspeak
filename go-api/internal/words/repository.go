package words

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation is the PostgreSQL error code for a unique constraint error.
const pgUniqueViolation = "23505"

// PostgresRepository stores words in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a repository that uses the given connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a word. It returns ErrDuplicate if the word already exists
// (case-insensitive, see the words_word_lower_idx index).
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (Word, error) {
	const query = `
		INSERT INTO words (word, translation, example)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	w := Word{Word: in.Word, Translation: in.Translation, Example: in.Example}
	err := r.pool.QueryRow(ctx, query, in.Word, in.Translation, in.Example).Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return Word{}, ErrDuplicate
		}
		return Word{}, fmt.Errorf("insert word: %w", err)
	}
	return w, nil
}

// List returns all words, oldest first.
func (r *PostgresRepository) List(ctx context.Context) ([]Word, error) {
	const query = `SELECT id, word, translation, example, created_at FROM words ORDER BY id`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list words: %w", err)
	}
	defer rows.Close()

	words := []Word{} // not nil: an empty list is encoded as [] in JSON, not null
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Word, &w.Translation, &w.Example, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		words = append(words, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list words: %w", err)
	}
	return words, nil
}

// Get returns one word by id, or ErrNotFound.
func (r *PostgresRepository) Get(ctx context.Context, id int64) (Word, error) {
	const query = `SELECT id, word, translation, example, created_at FROM words WHERE id = $1`

	var w Word
	err := r.pool.QueryRow(ctx, query, id).Scan(&w.ID, &w.Word, &w.Translation, &w.Example, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Word{}, ErrNotFound
	}
	if err != nil {
		return Word{}, fmt.Errorf("get word %d: %w", id, err)
	}
	return w, nil
}

// Delete removes one word by id, or returns ErrNotFound.
func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM words WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete word %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
