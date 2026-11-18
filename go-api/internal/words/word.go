// Package words implements the vocabulary words feature:
// HTTP handler → service → repository.
package words

import (
	"errors"
	"time"
)

// Word is one vocabulary entry.
type Word struct {
	ID          int64     `json:"id"`
	Word        string    `json:"word"`
	Translation string    `json:"translation"`
	Example     string    `json:"example"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateInput is the data needed to create a word.
type CreateInput struct {
	Word        string `json:"word"`
	Translation string `json:"translation"`
	Example     string `json:"example"`
}

// Sentinel errors. Callers check them with errors.Is.
var (
	ErrNotFound     = errors.New("word not found")
	ErrDuplicate    = errors.New("word already exists")
	ErrInvalidInput = errors.New("invalid input")
)
