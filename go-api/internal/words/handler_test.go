package words

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
)

// fakeService lets each test decide what the service returns.
// A nil function means "this method must not be called".
type fakeService struct {
	createFn func(CreateInput) (Word, error)
	listFn   func() ([]Word, error)
	getFn    func(int64) (Word, error)
	deleteFn func(int64) error
}

func (f *fakeService) Create(_ context.Context, in CreateInput) (Word, error) { return f.createFn(in) }
func (f *fakeService) List(_ context.Context) ([]Word, error)                 { return f.listFn() }
func (f *fakeService) Get(_ context.Context, id int64) (Word, error)          { return f.getFn(id) }
func (f *fakeService) Delete(_ context.Context, id int64) error               { return f.deleteFn(id) }

var errDB = errors.New("db is down")

func TestHandler(t *testing.T) {
	goWord := Word{ID: 1, Word: "Go", Translation: "Го"}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		svc        *fakeService
		wantStatus int
		wantBody   string // substring of the response body; "" = don't check
	}{
		// POST /words
		{
			name: "create: 201", method: http.MethodPost, path: "/words",
			body: `{"word":"Go","translation":"Го"}`,
			svc: &fakeService{createFn: func(in CreateInput) (Word, error) {
				return Word{ID: 1, Word: in.Word, Translation: in.Translation}, nil
			}},
			wantStatus: http.StatusCreated, wantBody: `"id":1`,
		},
		{
			name: "create: bad JSON → 400", method: http.MethodPost, path: "/words",
			body: `{"word":`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "invalid JSON body",
		},
		{
			name: "create: unknown field → 400", method: http.MethodPost, path: "/words",
			body: `{"word":"Go","translation":"Го","level":5}`, svc: &fakeService{},
			wantStatus: http.StatusBadRequest, wantBody: "invalid JSON body",
		},
		{
			name: "create: invalid input → 400", method: http.MethodPost, path: "/words",
			body: `{}`,
			svc: &fakeService{createFn: func(CreateInput) (Word, error) {
				return Word{}, fmt.Errorf("%w: word is required", ErrInvalidInput)
			}},
			wantStatus: http.StatusBadRequest, wantBody: "word is required",
		},
		{
			name: "create: duplicate → 409", method: http.MethodPost, path: "/words",
			body: `{"word":"go","translation":"идти"}`,
			svc: &fakeService{createFn: func(CreateInput) (Word, error) {
				return Word{}, ErrDuplicate
			}},
			wantStatus: http.StatusConflict, wantBody: "already exists",
		},
		{
			name: "create: unknown error → 500, details hidden", method: http.MethodPost, path: "/words",
			body: `{"word":"Go","translation":"Го"}`,
			svc: &fakeService{createFn: func(CreateInput) (Word, error) {
				return Word{}, errDB
			}},
			wantStatus: http.StatusInternalServerError, wantBody: `"internal error"`,
		},

		// GET /words
		{
			name: "list: 200", method: http.MethodGet, path: "/words",
			svc:        &fakeService{listFn: func() ([]Word, error) { return []Word{goWord}, nil }},
			wantStatus: http.StatusOK, wantBody: `"word":"Go"`,
		},
		{
			name: "list: empty → []", method: http.MethodGet, path: "/words",
			svc:        &fakeService{listFn: func() ([]Word, error) { return []Word{}, nil }},
			wantStatus: http.StatusOK, wantBody: "[]",
		},

		// GET /words/{id}
		{
			name: "get: 200", method: http.MethodGet, path: "/words/1",
			svc:        &fakeService{getFn: func(int64) (Word, error) { return goWord, nil }},
			wantStatus: http.StatusOK, wantBody: `"translation":"Го"`,
		},
		{
			name: "get: not found → 404", method: http.MethodGet, path: "/words/999",
			svc:        &fakeService{getFn: func(int64) (Word, error) { return Word{}, ErrNotFound }},
			wantStatus: http.StatusNotFound, wantBody: "not found",
		},
		{
			name: "get: id is not a number → 400", method: http.MethodGet, path: "/words/abc",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},
		{
			name: "get: id is zero → 400", method: http.MethodGet, path: "/words/0",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},

		// DELETE /words/{id}
		{
			name: "delete: 204", method: http.MethodDelete, path: "/words/1",
			svc:        &fakeService{deleteFn: func(int64) error { return nil }},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "delete: not found → 404", method: http.MethodDelete, path: "/words/999",
			svc:        &fakeService{deleteFn: func(int64) error { return ErrNotFound }},
			wantStatus: http.StatusNotFound, wantBody: "not found",
		},
		{
			name: "delete: bad id → 400", method: http.MethodDelete, path: "/words/-5",
			svc: &fakeService{}, wantStatus: http.StatusBadRequest, wantBody: "positive number",
		},

		// Routing
		{
			name: "PUT is not allowed → 405", method: http.MethodPut, path: "/words/1",
			svc: &fakeService{}, wantStatus: http.StatusMethodNotAllowed,
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
	svc := &fakeService{getFn: func(id int64) (Word, error) {
		gotID = id
		return Word{ID: id}, nil
	}}
	h := NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/words/42", nil))

	if gotID != 42 {
		t.Fatalf("service got id %d, want 42", gotID)
	}
	var w Word
	if err := json.NewDecoder(rec.Body).Decode(&w); err != nil || w.ID != 42 {
		t.Fatalf("response = %+v, %v; want ID 42", w, err)
	}
}
