package bot

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/dmitrycham/devspeak/go-api/internal/cards"
	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

func TestParseAdd(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    cards.CreateInput
		wantErr error
	}{
		{
			name: "em dash",
			text: "/add It stops accepting new requests — перестаёт принимать новые запросы",
			want: cards.CreateInput{Phrase: "It stops accepting new requests", Translation: "перестаёт принимать новые запросы"},
		},
		{
			name: "spaced hyphen",
			text: "/add roll back - откатить",
			want: cards.CreateInput{Phrase: "roll back", Translation: "откатить"},
		},
		{
			name: "extra spaces",
			text: "/add   roll back   —   откатить  ",
			want: cards.CreateInput{Phrase: "roll back", Translation: "откатить"},
		},
		{name: "no separator", text: "/add roll back", wantErr: ErrBadAdd},
		{name: "empty phrase", text: "/add — откатить", wantErr: ErrBadAdd},
		{name: "empty translation", text: "/add roll back —", wantErr: ErrBadAdd},
		{name: "only command", text: "/add", wantErr: ErrBadAdd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAdd(tt.text)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseAdd() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseAdd() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseAddAll(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    []cards.CreateInput
		wantErr error
	}{
		{
			name: "one line",
			text: "/add roll back — откатить",
			want: []cards.CreateInput{{Phrase: "roll back", Translation: "откатить"}},
		},
		{
			name: "three lines",
			text: "/add Could you clarify that? — не могли бы вы уточнить?\n" +
				"/add Let me think — дайте подумать\n" +
				"/add roll back — откатить",
			want: []cards.CreateInput{
				{Phrase: "Could you clarify that?", Translation: "не могли бы вы уточнить?"},
				{Phrase: "Let me think", Translation: "дайте подумать"},
				{Phrase: "roll back", Translation: "откатить"},
			},
		},
		{
			name: "blank lines and spaces are ignored",
			text: "\n  /add roll back — откатить  \n\n/add retry - повторить\n",
			want: []cards.CreateInput{
				{Phrase: "roll back", Translation: "откатить"},
				{Phrase: "retry", Translation: "повторить"},
			},
		},
		{name: "line without /add", text: "/add roll back — откатить\nretry — повторить", wantErr: ErrBadAdd},
		{name: "bad second line", text: "/add roll back — откатить\n/add retry", wantErr: ErrBadAdd},
		{name: "empty text", text: "", wantErr: ErrBadAdd},
		{name: "other command that starts with add", text: "/addx foo — bar", wantErr: ErrBadAdd},
		{name: "only command", text: "/add", wantErr: ErrBadAdd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAddAll(tt.text)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseAddAll() error = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseAddAll() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseAddAllErrorNamesTheLine(t *testing.T) {
	_, err := ParseAddAll("/add roll back — откатить\n/add retry")
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error = %v, want it to mention line 2", err)
	}
}

func TestGradeLabel(t *testing.T) {
	tests := []struct {
		grade srs.Grade
		want  string
	}{
		{grade: srs.Knew, want: "✅ Knew"},
		{grade: srs.Forgot, want: "❌ Forgot"},
	}

	for _, tt := range tests {
		t.Run(string(tt.grade), func(t *testing.T) {
			if got := gradeLabel(tt.grade); got != tt.want {
				t.Errorf("gradeLabel(%q) = %q, want %q", tt.grade, got, tt.want)
			}
		})
	}
}

func TestAddReply(t *testing.T) {
	tests := []struct {
		name       string
		saved      int
		duplicates []string
		want       string
	}{
		{name: "saved only", saved: 3, want: "Saved: 3"},
		{name: "duplicates only", duplicates: []string{"roll back"}, want: "Already saved: roll back"},
		{name: "both", saved: 2, duplicates: []string{"a", "b"}, want: "Saved: 2\nAlready saved: a; b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addReply(tt.saved, tt.duplicates); got != tt.want {
				t.Errorf("addReply() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseShow(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    int64
		wantErr bool
	}{
		{name: "ok", data: "show:7", want: 7},
		{name: "round trip", data: showData(123), want: 123},
		{name: "wrong prefix", data: "grade:7:knew", wantErr: true},
		{name: "not a number", data: "show:abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseShow(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseShow() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseShow() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseGrade(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantID    int64
		wantGrade srs.Grade
		wantErr   bool
	}{
		{name: "knew", data: "grade:7:knew", wantID: 7, wantGrade: srs.Knew},
		{name: "forgot", data: "grade:7:forgot", wantID: 7, wantGrade: srs.Forgot},
		{name: "round trip", data: gradeData(55, srs.Forgot), wantID: 55, wantGrade: srs.Forgot},
		{name: "wrong prefix", data: "show:7", wantErr: true},
		{name: "no grade", data: "grade:7", wantErr: true},
		{name: "unknown grade", data: "grade:7:maybe", wantErr: true},
		{name: "bad id", data: "grade:x:knew", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, grade, err := parseGrade(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseGrade() error = %v, wantErr %v", err, tt.wantErr)
			}
			if id != tt.wantID || grade != tt.wantGrade {
				t.Errorf("parseGrade() = (%d, %q), want (%d, %q)", id, grade, tt.wantID, tt.wantGrade)
			}
		})
	}
}
