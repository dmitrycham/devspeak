package cards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmitrycham/devspeak/go-api/internal/ai"
	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// fakeService lets each test decide what the service returns.
// A nil function means "this method must not be called".
type fakeService struct {
	createFn func(CreateInput) (Card, error)
	listFn   func() ([]Card, error)
	getFn    func(int64) (Card, error)
	deleteFn func(int64) error
	nextFn   func() (Card, error)
	reviewFn func(int64, srs.Grade) (Card, error)
	enrichFn func(int64) (Card, error)
}

func (f *fakeService) Create(_ context.Context, in CreateInput) (Card, error) { return f.createFn(in) }
func (f *fakeService) List(_ context.Context) ([]Card, error)                 { return f.listFn() }
func (f *fakeService) Get(_ context.Context, id int64) (Card, error)          { return f.getFn(id) }
func (f *fakeService) Delete(_ context.Context, id int64) error               { return f.deleteFn(id) }
func (f *fakeService) NextDue(_ context.Context) (Card, error)                { return f.nextFn() }
func (f *fakeService) Review(_ context.Context, id int64, g srs.Grade) (Card, error) {
	return f.reviewFn(id, g)
}
func (f *fakeService) Enrich(_ context.Context, id int64) (Card, error) { return f.enrichFn(id) }

var errDB = errors.New("db is down")

func TestHandler(t *testing.T) {
	goCard := Card{ID: 1, Phrase: "Go", Translation: "Го"}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		svc        *fakeService
		wantStatus int
		wantBody   string // substring of the response body; "" = don't check
	}{
		// POST /cards
		{
			name: "create: 201", method: http.MethodPost, path: "/cards",
			body: `{"phrase":"Go","translation":"Го"}`,
			svc: &fakeService{createFn: func(in CreateInput) (Card, error) {
				return Card{ID: 1, Phrase: in.Phrase, Translation: in.Translation}, nil
			}},
			wantStatus: http.StatusCreated, wantBody: `"id":1`,
		},
		{
			name: "create: bad JSON → 400", method: http.MethodPost, path: "/cards",
			body: `{"phrase":`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "invalid JSON body",
		},
		{
			name: "create: old field name 'word' → 400", method: http.MethodPost, path: "/cards",
			body: `{"word":"Go","translation":"Го"}`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "invalid JSON body",
		},
		{
			name: "create: invalid input → 400", method: http.MethodPost, path: "/cards",
			body: `{}`,
			svc: &fakeService{createFn: func(CreateInput) (Card, error) {
				return Card{}, fmt.Errorf("%w: phrase is required", ErrInvalidInput)
			}},
			wantStatus: http.StatusBadRequest, wantBody: "phrase is required",
		},
		{
			name: "create: duplicate → 409", method: http.MethodPost, path: "/cards",
			body: `{"phrase":"go","translation":"идти"}`,
			svc: &fakeService{createFn: func(CreateInput) (Card, error) {
				return Card{}, ErrDuplicate
			}},
			wantStatus: http.StatusConflict, wantBody: "already exists",
		},
		{
			name: "create: unknown error → 500, details hidden", method: http.MethodPost, path: "/cards",
			body: `{"phrase":"Go","translation":"Го"}`,
			svc: &fakeService{createFn: func(CreateInput) (Card, error) {
				return Card{}, errDB
			}},
			wantStatus: http.StatusInternalServerError, wantBody: `"internal error"`,
		},

		// GET /cards
		{
			name: "list: 200", method: http.MethodGet, path: "/cards",
			svc:        &fakeService{listFn: func() ([]Card, error) { return []Card{goCard}, nil }},
			wantStatus: http.StatusOK, wantBody: `"phrase":"Go"`,
		},
		{
			name: "list: empty → []", method: http.MethodGet, path: "/cards",
			svc:        &fakeService{listFn: func() ([]Card, error) { return []Card{}, nil }},
			wantStatus: http.StatusOK, wantBody: "[]",
		},

		// GET /cards/{id}
		{
			name: "get: 200", method: http.MethodGet, path: "/cards/1",
			svc:        &fakeService{getFn: func(int64) (Card, error) { return goCard, nil }},
			wantStatus: http.StatusOK, wantBody: `"translation":"Го"`,
		},
		{
			name: "get: not found → 404", method: http.MethodGet, path: "/cards/999",
			svc:        &fakeService{getFn: func(int64) (Card, error) { return Card{}, ErrNotFound }},
			wantStatus: http.StatusNotFound, wantBody: "not found",
		},
		{
			name: "get: id is not a number → 400", method: http.MethodGet, path: "/cards/abc",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},
		{
			name: "get: id is zero → 400", method: http.MethodGet, path: "/cards/0",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},

		// DELETE /cards/{id}
		{
			name: "delete: 204", method: http.MethodDelete, path: "/cards/1",
			svc:        &fakeService{deleteFn: func(int64) error { return nil }},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "delete: not found → 404", method: http.MethodDelete, path: "/cards/999",
			svc:        &fakeService{deleteFn: func(int64) error { return ErrNotFound }},
			wantStatus: http.StatusNotFound, wantBody: "not found",
		},
		{
			name: "delete: bad id → 400", method: http.MethodDelete, path: "/cards/-5",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},

		// POST /cards/{id}/enrich
		{
			name: "enrich: 200 with updated card", method: http.MethodPost, path: "/cards/1/enrich",
			svc: &fakeService{enrichFn: func(id int64) (Card, error) {
				return Card{ID: id, Phrase: "Go", ITContext: "A programming language."}, nil
			}},
			wantStatus: http.StatusOK, wantBody: `"it_context":"A programming language."`,
		},
		{
			name: "enrich: not found → 404", method: http.MethodPost, path: "/cards/999/enrich",
			svc:        &fakeService{enrichFn: func(int64) (Card, error) { return Card{}, ErrNotFound }},
			wantStatus: http.StatusNotFound, wantBody: "not found",
		},
		{
			name: "enrich: AI service down → 502, details hidden", method: http.MethodPost, path: "/cards/1/enrich",
			svc: &fakeService{enrichFn: func(int64) (Card, error) {
				return Card{}, fmt.Errorf("enrich card 1: %w: dial tcp 127.0.0.1:8000: connection refused", ai.ErrUnavailable)
			}},
			wantStatus: http.StatusBadGateway, wantBody: `"ai service unavailable"`,
		},
		{
			name: "enrich: AI timeout → 504", method: http.MethodPost, path: "/cards/1/enrich",
			svc: &fakeService{enrichFn: func(int64) (Card, error) {
				return Card{}, fmt.Errorf("enrich card 1: %w: status 504", ai.ErrTimeout)
			}},
			wantStatus: http.StatusGatewayTimeout, wantBody: `"ai service timed out"`,
		},
		{
			name: "enrich: bad id → 400", method: http.MethodPost, path: "/cards/abc/enrich",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},
		{
			name: "enrich: GET is not allowed → 405", method: http.MethodGet, path: "/cards/1/enrich",
			svc: &fakeService{}, wantStatus: http.StatusMethodNotAllowed,
		},

		// GET /reviews/next
		{
			name: "next: 200", method: http.MethodGet, path: "/reviews/next",
			svc:        &fakeService{nextFn: func() (Card, error) { return goCard, nil }},
			wantStatus: http.StatusOK, wantBody: `"phrase":"Go"`,
		},
		{
			name: "next: nothing due → 204", method: http.MethodGet, path: "/reviews/next",
			svc:        &fakeService{nextFn: func() (Card, error) { return Card{}, ErrNotFound }},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "next: unknown error → 500", method: http.MethodGet, path: "/reviews/next",
			svc:        &fakeService{nextFn: func() (Card, error) { return Card{}, errDB }},
			wantStatus: http.StatusInternalServerError, wantBody: `"internal error"`,
		},

		// POST /reviews/{id}
		{
			name: "review: knew → 200", method: http.MethodPost, path: "/reviews/1",
			body: `{"grade":"knew"}`,
			svc: &fakeService{reviewFn: func(id int64, g srs.Grade) (Card, error) {
				return Card{ID: id, Repetitions: 1, IntervalDays: 1}, nil
			}},
			wantStatus: http.StatusOK, wantBody: `"interval_days":1`,
		},
		{
			name: "review: bad grade → 400", method: http.MethodPost, path: "/reviews/1",
			body: `{"grade":"easy"}`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "grade must be knew or forgot",
		},
		{
			name: "review: missing grade → 400", method: http.MethodPost, path: "/reviews/1",
			body: `{}`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "grade must be knew or forgot",
		},
		{
			name: "review: unknown field → 400", method: http.MethodPost, path: "/reviews/1",
			body: `{"grade":"knew","score":5}`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "invalid JSON body",
		},
		{
			name: "review: bad id → 400", method: http.MethodPost, path: "/reviews/abc",
			body: `{"grade":"knew"}`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},
		{
			name: "review: card not found → 404", method: http.MethodPost, path: "/reviews/999",
			body: `{"grade":"forgot"}`,
			svc: &fakeService{reviewFn: func(int64, srs.Grade) (Card, error) {
				return Card{}, ErrNotFound
			}},
			wantStatus: http.StatusNotFound, wantBody: "not found",
		},

		// Routing
		{
			name: "PUT is not allowed → 405", method: http.MethodPut, path: "/cards/1",
			svc: &fakeService{}, wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name: "old path /words → 404", method: http.MethodGet, path: "/words",
			svc: &fakeService{}, wantStatus: http.StatusNotFound,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.svc, logger)
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.Routes().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %s, want it to contain %s", rec.Body, tt.wantBody)
			}
		})
	}
}

// The handler must pass the id from the path to the service.
func TestHandler_PassesIDToService(t *testing.T) {
	var gotID int64
	svc := &fakeService{getFn: func(id int64) (Card, error) {
		gotID = id
		return Card{ID: id}, nil
	}}
	h := NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cards/42", nil))

	if gotID != 42 {
		t.Fatalf("service got id %d, want 42", gotID)
	}
	var c Card
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil || c.ID != 42 {
		t.Fatalf("response = %+v, %v; want ID 42", c, err)
	}
}

// The review handler must pass the id and the parsed grade to the service.
func TestHandler_ReviewPassesIDAndGrade(t *testing.T) {
	var gotID int64
	var gotGrade srs.Grade
	svc := &fakeService{reviewFn: func(id int64, g srs.Grade) (Card, error) {
		gotID, gotGrade = id, g
		return Card{ID: id}, nil
	}}
	h := NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/reviews/7", strings.NewReader(`{"grade":"forgot"}`))
	h.Routes().ServeHTTP(rec, req)

	if gotID != 7 || gotGrade != srs.Forgot {
		t.Fatalf("service got (%d, %q), want (7, %q)", gotID, gotGrade, srs.Forgot)
	}
}
