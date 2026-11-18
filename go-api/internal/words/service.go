package words

import (
	"context"
	"fmt"
	"strings"
)

// Repository is the storage that Service needs.
// It is declared here, by the consumer, so tests can pass a fake.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (Word, error)
	List(ctx context.Context) ([]Word, error)
	Get(ctx context.Context, id int64) (Word, error)
	Delete(ctx context.Context, id int64) error
}

// Service holds the business rules for words.
type Service struct {
	repo Repository
}

// NewService creates a Service that stores words in repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates the input and saves a new word.
//
// Rules:
//   - Trim spaces in Word, Translation and Example (strings.TrimSpace).
//   - Word and Translation are required. If one is empty after trimming,
//     return an error that wraps ErrInvalidInput, for example:
//     fmt.Errorf("%w: word is required", ErrInvalidInput)
//     Do not call the repository in this case.
//   - Call s.repo.Create with the trimmed input.
//   - Return repository errors (like ErrDuplicate) as they are, or wrapped with %w.
//
// Check your code: go test ./internal/words -run TestService -v
func (s *Service) Create(ctx context.Context, in CreateInput) (Word, error) {
	in.Word = strings.TrimSpace(in.Word)
	in.Translation = strings.TrimSpace(in.Translation)
	in.Example = strings.TrimSpace(in.Example)

	if in.Word == "" {
		return Word{}, fmt.Errorf("%w: word is required", ErrInvalidInput)
	}
	if in.Translation == "" {
		return Word{}, fmt.Errorf("%w: translation is required", ErrInvalidInput)
	}

	return s.repo.Create(ctx, in)
}

// List returns all words.
func (s *Service) List(ctx context.Context) ([]Word, error) {
	return s.repo.List(ctx)
}

// Get returns one word by id, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id int64) (Word, error) {
	return s.repo.Get(ctx, id)
}

// Delete removes one word by id, or returns ErrNotFound.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
