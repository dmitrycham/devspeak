// Package srs implements spaced repetition (a simplified SM-2) with two grades.
// It is pure: no database and no clock, so it is easy to test.
package srs

import (
	"errors"
	"fmt"
	"math"
)

// Grade is the user's answer for a card.
type Grade string

// The two grades.
const (
	Knew   Grade = "knew"
	Forgot Grade = "forgot"
)

// MinEase is the lowest allowed ease factor.
const MinEase = 1.3

// ErrInvalidGrade means the grade is not "knew" or "forgot".
var ErrInvalidGrade = errors.New("grade must be knew or forgot")

// ParseGrade converts a string from the API into a Grade.
func ParseGrade(s string) (Grade, error) {
	switch g := Grade(s); g {
	case Knew, Forgot:
		return g, nil
	default:
		return "", fmt.Errorf("%w: got %q", ErrInvalidGrade, s)
	}
}

// State is the spaced repetition state of one card.
type State struct {
	Repetitions  int     // successful reviews in a row
	IntervalDays int     // days until the next review
	Ease         float64 // how fast the interval grows (starts at 2.5)
}

// Next returns the new state after a review.
//
// knew:
//   - IntervalDays: 1 if Repetitions == 0, 3 if Repetitions == 1,
//     otherwise round(IntervalDays * Ease) (use math.Round).
//   - Repetitions + 1. Ease does not change.
//
// forgot:
//   - Repetitions = 0, IntervalDays = 0 (the card is due again now).
//   - Ease - 0.2, but never below MinEase.
func Next(s State, g Grade) State {
	switch g {
	case Knew:
		switch s.Repetitions {
		case 0:
			s.IntervalDays = 1
		case 1:
			s.IntervalDays = 3
		default:
			s.IntervalDays = int(math.Round(float64(s.IntervalDays) * s.Ease))
		}
		s.Repetitions++
	case Forgot:
		s.Repetitions = 0
		s.IntervalDays = 0
		s.Ease = max(MinEase, s.Ease-0.2)
	}
	return s
}
