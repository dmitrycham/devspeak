// Package bot is the Telegram interface of DevSpeak.
// It talks to the user and calls the cards service. It has no SQL.
package bot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dmitrycham/devspeak/go-api/internal/cards"
	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// Callback data is limited to 64 bytes by Telegram, so we keep it short.
const (
	showPrefix  = "show:"
	gradePrefix = "grade:"
)

// ErrBadAdd means the /add text does not look like "phrase — translation".
var ErrBadAdd = errors.New("use: /add phrase — translation")

// ErrBadCallback means the button data is not one we created.
var ErrBadCallback = errors.New("bad callback data")

// ParseAdd turns "/add phrase — translation" into a card input.
// The separator is an em dash; " - " also works, because it is easier to type.
func ParseAdd(text string) (cards.CreateInput, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/add"))

	phrase, translation, found := strings.Cut(rest, "—")
	if !found {
		phrase, translation, found = strings.Cut(rest, " - ")
	}

	phrase = strings.TrimSpace(phrase)
	translation = strings.TrimSpace(translation)
	if !found || phrase == "" || translation == "" {
		return cards.CreateInput{}, ErrBadAdd
	}

	return cards.CreateInput{Phrase: phrase, Translation: translation}, nil
}

// ParseAddAll reads one or more "/add phrase — translation" lines from one message.
// Every non-empty line must start with /add, so a pasted list can never glue
// several phrases into one card. Nothing is saved if any line is wrong.
func ParseAddAll(text string) ([]cards.CreateInput, error) {
	var inputs []cards.CreateInput

	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "/add") {
			return nil, fmt.Errorf("line %d: %w", i+1, ErrBadAdd)
		}
		in, err := ParseAdd(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		inputs = append(inputs, in)
	}

	if len(inputs) == 0 {
		return nil, ErrBadAdd
	}
	return inputs, nil
}

// addReply builds the answer to /add: how many cards were saved
// and which phrases were skipped because they already exist.
func addReply(saved int, duplicates []string) string {
	var parts []string
	if saved > 0 {
		parts = append(parts, fmt.Sprintf("Saved: %d", saved))
	}
	if len(duplicates) > 0 {
		parts = append(parts, "Already saved: "+strings.Join(duplicates, "; "))
	}
	return strings.Join(parts, "\n")
}

// showData is the callback data of the "Show" button.
func showData(id int64) string {
	return showPrefix + strconv.FormatInt(id, 10)
}

// gradeData is the callback data of the "Knew" and "Forgot" buttons.
func gradeData(id int64, g srs.Grade) string {
	return fmt.Sprintf("%s%d:%s", gradePrefix, id, g)
}

// parseShow reads the card id from the "Show" callback data.
func parseShow(data string) (int64, error) {
	raw, ok := strings.CutPrefix(data, showPrefix)
	if !ok {
		return 0, ErrBadCallback
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadCallback, data)
	}
	return id, nil
}

// parseGrade reads the card id and the grade from the grade callback data.
func parseGrade(data string) (int64, srs.Grade, error) {
	raw, ok := strings.CutPrefix(data, gradePrefix)
	if !ok {
		return 0, "", ErrBadCallback
	}
	rawID, rawGrade, ok := strings.Cut(raw, ":")
	if !ok {
		return 0, "", fmt.Errorf("%w: %q", ErrBadCallback, data)
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %q", ErrBadCallback, data)
	}
	grade, err := srs.ParseGrade(rawGrade)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", ErrBadCallback, err)
	}
	return id, grade, nil
}
