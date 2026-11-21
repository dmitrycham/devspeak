package cards

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/dmitrycham/devspeak/go-api/internal/ai"
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

	enrichCalled    bool
	enrichExample   string
	enrichITContext string
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

func (f *fakeRepo) UpdateEnrichment(_ context.Context, id int64, example, itContext string) (Card, error) {
	f.enrichCalled = true
	f.enrichExample = example
	f.enrichITContext = itContext
	return Card{ID: id, Phrase: f.card.Phrase, Example: example, ITContext: itContext}, nil
}

// fakeEnricher returns a fixed result and records the phrase it received.
type fakeEnricher struct {
	result ai.Enrichment
	err    error

	called    bool
	gotPhrase string
}

func (f *fakeEnricher) Enrich(_ context.Context, phrase string) (ai.Enrichment, error) {
	f.called = true
	f.gotPhrase = phrase
	return f.result, f.err
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
			svc := NewService(repo, nil)

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
		svc := NewService(&fakeRepo{card: Card{ID: 7, Phrase: "defer"}}, nil)
		got, err := svc.Get(context.Background(), 7)
		if err != nil || got.ID != 7 {
			t.Fatalf("Get = %+v, %v; want ID 7, nil", got, err)
		}
	})

	t.Run("List returns the repo cards", func(t *testing.T) {
		svc := NewService(&fakeRepo{cards: []Card{{ID: 1}, {ID: 2}}}, nil)
		got, err := svc.List(context.Background())
		if err != nil || len(got) != 2 {
			t.Fatalf("List = %v, %v; want 2 cards, nil", got, err)
		}
	})

	t.Run("errors are passed through", func(t *testing.T) {
		svc := NewService(&fakeRepo{err: repoErr}, nil)
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
	svc := NewService(repo, nil)
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
			svc := NewService(repo, nil)
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
			svc := NewService(repo, nil)

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
	svc := NewService(repo, nil)

	_, err := svc.Review(context.Background(), 99, srs.Knew)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if repo.updateCalled {
		t.Error("UpdateSchedule must not be called for a missing card")
	}
}

func TestService_Enrich(t *testing.T) {
	aiResult := ai.Enrichment{Translation: "выкатить", Example: "We roll out on Friday.", ITContext: "Release a new version."}

	tests := []struct {
		name       string
		repo       *fakeRepo
		enricher   *fakeEnricher
		wantErr    error // nil = success
		wantAICall bool
		wantUpdate bool
		wantCard   Card
	}{
		{
			name:       "new card → AI call and update",
			repo:       &fakeRepo{card: Card{ID: 1, Phrase: "roll out"}},
			enricher:   &fakeEnricher{result: aiResult},
			wantAICall: true, wantUpdate: true,
			wantCard: Card{ID: 1, Phrase: "roll out", Example: aiResult.Example, ITContext: aiResult.ITContext},
		},
		{
			name:     "already enriched → card as is, no AI call",
			repo:     &fakeRepo{card: Card{ID: 1, Phrase: "roll out", ITContext: "old context"}},
			enricher: &fakeEnricher{result: aiResult},
			wantCard: Card{ID: 1, Phrase: "roll out", ITContext: "old context"},
		},
		{
			name:     "card not found → ErrNotFound, no AI call",
			repo:     &fakeRepo{err: ErrNotFound},
			enricher: &fakeEnricher{result: aiResult},
			wantErr:  ErrNotFound,
		},
		{
			name:       "AI fails → error passed through, no update",
			repo:       &fakeRepo{card: Card{ID: 1, Phrase: "roll out"}},
			enricher:   &fakeEnricher{err: fmt.Errorf("%w: status 500", ai.ErrUnavailable)},
			wantErr:    ai.ErrUnavailable,
			wantAICall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, tt.enricher)

			got, err := svc.Enrich(context.Background(), 1)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.enricher.called != tt.wantAICall {
				t.Errorf("AI called = %v, want %v", tt.enricher.called, tt.wantAICall)
			}
			if tt.repo.enrichCalled != tt.wantUpdate {
				t.Errorf("UpdateEnrichment called = %v, want %v", tt.repo.enrichCalled, tt.wantUpdate)
			}
			if tt.wantAICall && tt.enricher.gotPhrase != "roll out" {
				t.Errorf("AI got phrase %q, want %q", tt.enricher.gotPhrase, "roll out")
			}
			if got != tt.wantCard {
				t.Errorf("card = %+v, want %+v", got, tt.wantCard)
			}
		})
	}
}
