package cards

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// Repository is the storage that Service needs.
// It is declared here, by the consumer, so tests can pass a fake.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (Card, error)
	List(ctx context.Context) ([]Card, error)
	Get(ctx context.Context, id int64) (Card, error)
	Delete(ctx context.Context, id int64) error
	NextDue(ctx context.Context, now time.Time) (Card, error)
	UpdateSchedule(ctx context.Context, id int64, st srs.State, dueAt time.Time) (Card, error)
}

// Service holds the business rules for cards and reviews.
type Service struct {
	repo Repository
	now  func() time.Time // the clock; tests replace it with a fixed time
}

// NewService creates a Service that stores cards in repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Create trims the input, checks required fields and saves a new card.
// It returns ErrInvalidInput if phrase or translation is empty,
// and ErrDuplicate if the phrase already exists.
func (s *Service) Create(ctx context.Context, in CreateInput) (Card, error) {
	in.Phrase = strings.TrimSpace(in.Phrase)
	in.Translation = strings.TrimSpace(in.Translation)
	in.Example = strings.TrimSpace(in.Example)

	if in.Phrase == "" {
		return Card{}, fmt.Errorf("%w: phrase is required", ErrInvalidInput)
	}
	if in.Translation == "" {
		return Card{}, fmt.Errorf("%w: translation is required", ErrInvalidInput)
	}

	return s.repo.Create(ctx, in)
}

// List returns all cards.
func (s *Service) List(ctx context.Context) ([]Card, error) {
	return s.repo.List(ctx)
}

// Get returns one card by id, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id int64) (Card, error) {
	return s.repo.Get(ctx, id)
}

// Delete removes one card by id, or returns ErrNotFound.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// NextDue returns the next card to review, or ErrNotFound if none is due.
func (s *Service) NextDue(ctx context.Context) (Card, error) {
	return s.repo.NextDue(ctx, s.now())
}

// Review applies the grade to a card: it computes the new SM-2 state
// and the next due time, and saves them. It returns ErrInvalidInput for
// an unknown grade and ErrNotFound if the card does not exist.
func (s *Service) Review(ctx context.Context, id int64, grade srs.Grade) (Card, error) {
	// Check the grade here too: not every caller goes through the HTTP handler.
	if _, err := srs.ParseGrade(string(grade)); err != nil {
		return Card{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	card, err := s.repo.Get(ctx, id)
	if err != nil {
		return Card{}, err
	}

	next := srs.Next(card.State(), grade)
	dueAt := s.now().AddDate(0, 0, next.IntervalDays) // interval 0 → due now

	return s.repo.UpdateSchedule(ctx, id, next, dueAt)
}
