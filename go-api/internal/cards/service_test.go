package cards

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// fakeRepo is an in-memory Repository for service tests.
// It records what it received, so tests can check the input.
type fakeRepo struct {
	createCalled bool
	createInput  CreateInput
	createErr    error

	card      Card
	cards     []Card
	err       error
	getCalled bool

	nextDueNow time.Time // the "now" that NextDue received

	updateCalled bool
	updateState  srs.State
	updateDueAt  time.Time
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (Card, error) {
	f.createCalled = true
	f.createInput = in
	if f.createErr != nil {
		return Card{}, f.createErr
	}
	return Card{ID: 1, Phrase: in.Phrase, Translation: in.Translation, Example: in.Example}, nil
}

func (f *fakeRepo) List(_ context.Context) ([]Card, error) { return f.cards, f.err }

func (f *fakeRepo) Get(_ context.Context, _ int64) (Card, error) {
	f.getCalled = true
	return f.card, f.err
}

func (f *fakeRepo) Delete(_ context.Context, _ int64) error { return f.err }

func (f *fakeRepo) NextDue(_ context.Context, now time.Time) (Card, error) {
	f.nextDueNow = now
	return f.card, f.err
}

func (f *fakeRepo) UpdateSchedule(_ context.Context, id int64, st srs.State, dueAt time.Time) (Card, error) {
	f.updateCalled = true
	f.updateState = st
	f.updateDueAt = dueAt
	return Card{ID: id, Repetitions: st.Repetitions, IntervalDays: st.IntervalDays, Ease: st.Ease, DueAt: dueAt}, nil
}

func TestService_Create(t *testing.T) {
	tests := []struct {
		name       string
		input      CreateInput
		repoErr    error
		wantErr    error       // nil means success
		wantSaved  CreateInput // what the repo must receive (only on success)
		wantRepoOK bool        // must the repo be called?
	}{
		{
			name:       "valid input",
			input:      CreateInput{Phrase: "goroutine", Translation: "горутина", Example: "Start a goroutine."},
			wantSaved:  CreateInput{Phrase: "goroutine", Translation: "горутина", Example: "Start a goroutine."},
			wantRepoOK: true,
		},
		{
			name:       "spaces are trimmed",
			input:      CreateInput{Phrase: "  channel ", Translation: "\tканал\n", Example: "  Send to a channel.  "},
			wantSaved:  CreateInput{Phrase: "channel", Translation: "канал", Example: "Send to a channel."},
			wantRepoOK: true,
		},
		{
			name:       "empty example is OK",
			input:      CreateInput{Phrase: "mutex", Translation: "мьютекс"},
			wantSaved:  CreateInput{Phrase: "mutex", Translation: "мьютекс"},
			wantRepoOK: true,
		},
		{
			name:    "missing phrase",
			input:   CreateInput{Translation: "перевод"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "missing translation",
			input:   CreateInput{Phrase: "slice"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "phrase is only spaces",
			input:   CreateInput{Phrase: "   ", Translation: "перевод"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "translation is only spaces",
			input:   CreateInput{Phrase: "slice", Translation: " \t "},
			wantErr: ErrInvalidInput,
		},
		{
			name:       "repo duplicate error is returned",
			input:      CreateInput{Phrase: "Go", Translation: "Го"},
			repoErr:    ErrDuplicate,
			wantErr:    ErrDuplicate,
			wantRepoOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{createErr: tt.repoErr}
			svc := NewService(repo)

			got, err := svc.Create(context.Background(), tt.input)

			if repo.createCalled != tt.wantRepoOK {
				t.Fatalf("repo.Create called = %v, want %v", repo.createCalled, tt.wantRepoOK)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.createInput != tt.wantSaved {
				t.Errorf("repo got %+v, want %+v", repo.createInput, tt.wantSaved)
			}
			if got.Phrase != tt.wantSaved.Phrase {
				t.Errorf("returned phrase = %q, want %q", got.Phrase, tt.wantSaved.Phrase)
			}
		})
	}
}

func TestService_PassThrough(t *testing.T) {
	repoErr := errors.New("db is down")

	t.Run("Get returns the repo card", func(t *testing.T) {
		svc := NewService(&fakeRepo{card: Card{ID: 7, Phrase: "defer"}})
		got, err := svc.Get(context.Background(), 7)
		if err != nil || got.ID != 7 {
			t.Fatalf("Get = %+v, %v; want ID 7, nil", got, err)
		}
	})

	t.Run("List returns the repo cards", func(t *testing.T) {
		svc := NewService(&fakeRepo{cards: []Card{{ID: 1}, {ID: 2}}})
		got, err := svc.List(context.Background())
		if err != nil || len(got) != 2 {
			t.Fatalf("List = %v, %v; want 2 cards, nil", got, err)
		}
	})

	t.Run("errors are passed through", func(t *testing.T) {
		svc := NewService(&fakeRepo{err: repoErr})
		if _, err := svc.Get(context.Background(), 1); !errors.Is(err, repoErr) {
			t.Errorf("Get error = %v, want %v", err, repoErr)
		}
		if _, err := svc.List(context.Background()); !errors.Is(err, repoErr) {
			t.Errorf("List error = %v, want %v", err, repoErr)
		}
		if err := svc.Delete(context.Background(), 1); !errors.Is(err, repoErr) {
			t.Errorf("Delete error = %v, want %v", err, repoErr)
		}
	})
}

// fixedNow is the fake clock time for review tests.
var fixedNow = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func TestService_NextDue_UsesClock(t *testing.T) {
	repo := &fakeRepo{card: Card{ID: 3}}
	svc := NewService(repo)
	svc.now = func() time.Time { return fixedNow }

	got, err := svc.NextDue(context.Background())

	if err != nil || got.ID != 3 {
		t.Fatalf("NextDue = %+v, %v; want ID 3, nil", got, err)
	}
	if !repo.nextDueNow.Equal(fixedNow) {
		t.Errorf("repo got now = %v, want %v", repo.nextDueNow, fixedNow)
	}
}

// These cases use the real srs.Next, so they pass only when srs.Next is implemented.
func TestService_Review(t *testing.T) {
	newCard := Card{ID: 5, Repetitions: 0, IntervalDays: 0, Ease: 2.5}

	tests := []struct {
		name      string
		card      Card
		grade     srs.Grade
		wantState srs.State
		wantDueAt time.Time
	}{
		{
			name:      "knew: due in 1 day",
			card:      newCard,
			grade:     srs.Knew,
			wantState: srs.State{Repetitions: 1, IntervalDays: 1, Ease: 2.5},
			wantDueAt: fixedNow.AddDate(0, 0, 1),
		},
		{
			name:      "forgot: due now",
			card:      Card{ID: 5, Repetitions: 3, IntervalDays: 15, Ease: 2.5},
			grade:     srs.Forgot,
			wantState: srs.State{Repetitions: 0, IntervalDays: 0, Ease: 2.3},
			wantDueAt: fixedNow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{card: tt.card}
			svc := NewService(repo)
			svc.now = func() time.Time { return fixedNow }

			_, err := svc.Review(context.Background(), tt.card.ID, tt.grade)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := repo.updateState
			if got.Repetitions != tt.wantState.Repetitions || got.IntervalDays != tt.wantState.IntervalDays ||
				math.Abs(got.Ease-tt.wantState.Ease) > 1e-9 {
				t.Errorf("saved state = %+v, want %+v", got, tt.wantState)
			}
			if !repo.updateDueAt.Equal(tt.wantDueAt) {
				t.Errorf("saved due_at = %v, want %v", repo.updateDueAt, tt.wantDueAt)
			}
		})
	}
}

func TestService_Review_InvalidGrade(t *testing.T) {
	tests := []srs.Grade{"", "easy", "KNEW"}

	for _, grade := range tests {
		t.Run(string(grade), func(t *testing.T) {
			repo := &fakeRepo{card: Card{ID: 5, Ease: 2.5}}
			svc := NewService(repo)

			_, err := svc.Review(context.Background(), 5, grade)

			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
			if !errors.Is(err, srs.ErrInvalidGrade) {
				t.Errorf("error = %v, want it to also wrap srs.ErrInvalidGrade", err)
			}
			if repo.getCalled || repo.updateCalled {
				t.Error("the repository must not be called for an invalid grade")
			}
		})
	}
}

func TestService_Review_CardNotFound(t *testing.T) {
	repo := &fakeRepo{err: ErrNotFound}
	svc := NewService(repo)

	_, err := svc.Review(context.Background(), 99, srs.Knew)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if repo.updateCalled {
		t.Error("UpdateSchedule must not be called for a missing card")
	}
}
