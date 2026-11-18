package words

import (
	"context"
	"errors"
	"testing"
)

// fakeRepo is an in-memory Repository for service tests.
// It records what Create received, so tests can check the input.
type fakeRepo struct {
	createCalled bool
	createInput  CreateInput
	createErr    error

	word  Word
	words []Word
	err   error
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (Word, error) {
	f.createCalled = true
	f.createInput = in
	if f.createErr != nil {
		return Word{}, f.createErr
	}
	return Word{ID: 1, Word: in.Word, Translation: in.Translation, Example: in.Example}, nil
}

func (f *fakeRepo) List(_ context.Context) ([]Word, error) { return f.words, f.err }

func (f *fakeRepo) Get(_ context.Context, _ int64) (Word, error) { return f.word, f.err }

func (f *fakeRepo) Delete(_ context.Context, _ int64) error { return f.err }

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
			input:      CreateInput{Word: "goroutine", Translation: "горутина", Example: "Start a goroutine."},
			wantSaved:  CreateInput{Word: "goroutine", Translation: "горутина", Example: "Start a goroutine."},
			wantRepoOK: true,
		},
		{
			name:       "spaces are trimmed",
			input:      CreateInput{Word: "  channel ", Translation: "\tканал\n", Example: "  Send to a channel.  "},
			wantSaved:  CreateInput{Word: "channel", Translation: "канал", Example: "Send to a channel."},
			wantRepoOK: true,
		},
		{
			name:       "empty example is OK",
			input:      CreateInput{Word: "mutex", Translation: "мьютекс"},
			wantSaved:  CreateInput{Word: "mutex", Translation: "мьютекс"},
			wantRepoOK: true,
		},
		{
			name:    "missing word",
			input:   CreateInput{Translation: "перевод"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "missing translation",
			input:   CreateInput{Word: "slice"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "word is only spaces",
			input:   CreateInput{Word: "   ", Translation: "перевод"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "translation is only spaces",
			input:   CreateInput{Word: "slice", Translation: " \t "},
			wantErr: ErrInvalidInput,
		},
		{
			name:       "repo duplicate error is returned",
			input:      CreateInput{Word: "Go", Translation: "Го"},
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
			if got.Word != tt.wantSaved.Word {
				t.Errorf("returned word = %q, want %q", got.Word, tt.wantSaved.Word)
			}
		})
	}
}

func TestService_PassThrough(t *testing.T) {
	repoErr := errors.New("db is down")

	t.Run("Get returns the repo word", func(t *testing.T) {
		svc := NewService(&fakeRepo{word: Word{ID: 7, Word: "defer"}})
		got, err := svc.Get(context.Background(), 7)
		if err != nil || got.ID != 7 {
			t.Fatalf("Get = %+v, %v; want ID 7, nil", got, err)
		}
	})

	t.Run("List returns the repo words", func(t *testing.T) {
		svc := NewService(&fakeRepo{words: []Word{{ID: 1}, {ID: 2}}})
		got, err := svc.List(context.Background())
		if err != nil || len(got) != 2 {
			t.Fatalf("List = %v, %v; want 2 words, nil", got, err)
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
