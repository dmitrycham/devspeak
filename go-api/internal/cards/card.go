// Package cards implements vocabulary cards with spaced repetition reviews:
// HTTP handler → service → repository.
package cards

import (
	"errors"
	"time"

	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// Card is one vocabulary card with its spaced repetition state.
type Card struct {
	ID           int64     `json:"id"`
	Phrase       string    `json:"phrase"`
	Translation  string    `json:"translation"`
	Example      string    `json:"example"`
	ITContext    string    `json:"it_context"` // "" until the card is enriched
	DueAt        time.Time `json:"due_at"`
	IntervalDays int       `json:"interval_days"`
	Ease         float64   `json:"ease"`
	Repetitions  int       `json:"repetitions"`
	CreatedAt    time.Time `json:"created_at"`
}

// State returns the card's spaced repetition state for the srs package.
func (c Card) State() srs.State {
	return srs.State{Repetitions: c.Repetitions, IntervalDays: c.IntervalDays, Ease: c.Ease}
}

// CreateInput is the data needed to create a card.
type CreateInput struct {
	Phrase      string `json:"phrase"`
	Translation string `json:"translation"`
	Example     string `json:"example"`
}

// Sentinel errors. Callers check them with errors.Is.
var (
	ErrNotFound     = errors.New("card not found")
	ErrDuplicate    = errors.New("card already exists")
	ErrInvalidInput = errors.New("invalid input")
)
