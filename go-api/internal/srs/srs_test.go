package srs

import (
	"errors"
	"math"
	"testing"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name  string
		state State
		grade Grade
		want  State
	}{
		// knew: 1 day → 3 days → interval × ease
		{
			name:  "new card, knew → 1 day",
			state: State{Repetitions: 0, IntervalDays: 0, Ease: 2.5},
			grade: Knew,
			want:  State{Repetitions: 1, IntervalDays: 1, Ease: 2.5},
		},
		{
			name:  "second review, knew → 3 days",
			state: State{Repetitions: 1, IntervalDays: 1, Ease: 2.5},
			grade: Knew,
			want:  State{Repetitions: 2, IntervalDays: 3, Ease: 2.5},
		},
		{
			name:  "third review, knew → interval × ease (3 × 2.5 = 7.5 → 8)",
			state: State{Repetitions: 2, IntervalDays: 3, Ease: 2.5},
			grade: Knew,
			want:  State{Repetitions: 3, IntervalDays: 8, Ease: 2.5},
		},
		{
			name:  "knew rounds to the nearest day (7 × 2.36 = 16.52 → 17)",
			state: State{Repetitions: 4, IntervalDays: 7, Ease: 2.36},
			grade: Knew,
			want:  State{Repetitions: 5, IntervalDays: 17, Ease: 2.36},
		},
		{
			name:  "knew does not change ease",
			state: State{Repetitions: 3, IntervalDays: 10, Ease: 1.3},
			grade: Knew,
			want:  State{Repetitions: 4, IntervalDays: 13, Ease: 1.3},
		},

		// forgot: start again, due now, ease − 0.2 (min 1.3)
		{
			name:  "forgot resets repetitions and interval, ease − 0.2",
			state: State{Repetitions: 5, IntervalDays: 40, Ease: 2.5},
			grade: Forgot,
			want:  State{Repetitions: 0, IntervalDays: 0, Ease: 2.3},
		},
		{
			name:  "forgot never takes ease below 1.3",
			state: State{Repetitions: 2, IntervalDays: 6, Ease: 1.4},
			grade: Forgot,
			want:  State{Repetitions: 0, IntervalDays: 0, Ease: MinEase},
		},
		{
			name:  "forgot at minimum ease stays at 1.3",
			state: State{Repetitions: 1, IntervalDays: 1, Ease: MinEase},
			grade: Forgot,
			want:  State{Repetitions: 0, IntervalDays: 0, Ease: MinEase},
		},
		{
			name:  "new card, forgot",
			state: State{Repetitions: 0, IntervalDays: 0, Ease: 2.5},
			grade: Forgot,
			want:  State{Repetitions: 0, IntervalDays: 0, Ease: 2.3},
		},
		{
			name:  "after forgot, knew → 1 day again",
			state: State{Repetitions: 0, IntervalDays: 0, Ease: 2.3},
			grade: Knew,
			want:  State{Repetitions: 1, IntervalDays: 1, Ease: 2.3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Next(tt.state, tt.grade)

			if got.Repetitions != tt.want.Repetitions {
				t.Errorf("Repetitions = %d, want %d", got.Repetitions, tt.want.Repetitions)
			}
			if got.IntervalDays != tt.want.IntervalDays {
				t.Errorf("IntervalDays = %d, want %d", got.IntervalDays, tt.want.IntervalDays)
			}
			// Floats are not always exact (2.5 - 0.2 may be 2.2999999...),
			// so compare with a small tolerance.
			if math.Abs(got.Ease-tt.want.Ease) > 1e-9 {
				t.Errorf("Ease = %v, want %v", got.Ease, tt.want.Ease)
			}
		})
	}
}

func TestParseGrade(t *testing.T) {
	tests := []struct {
		in      string
		want    Grade
		wantErr bool
	}{
		{in: "knew", want: Knew},
		{in: "forgot", want: Forgot},
		{in: "", wantErr: true},
		{in: "KNEW", wantErr: true},
		{in: "easy", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseGrade(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidGrade) {
					t.Fatalf("error = %v, want ErrInvalidGrade", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParseGrade(%q) = %q, %v; want %q, nil", tt.in, got, err, tt.want)
			}
		})
	}
}
