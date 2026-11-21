package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const okBody = `{"translation":"выкатить","example":"We roll out on Friday.","it_context":"Release a new version."}`

// slowHandler answers only after the client has given up (or after 5s as a safety net).
func slowHandler(w http.ResponseWriter, r *http.Request) {
	// The server notices a closed connection only after the body is read to EOF,
	// so without this r.Context() is never cancelled and every test waits 5s.
	_, _ = io.Copy(io.Discard, r.Body)
	select {
	case <-r.Context().Done():
	case <-time.After(5 * time.Second):
	}
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestClient_Enrich(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		timeout time.Duration
		wantErr error // nil = success
	}{
		{name: "200 → enrichment", handler: reply(http.StatusOK, okBody), timeout: time.Second},
		{name: "502 → unavailable", handler: reply(http.StatusBadGateway, `{"detail":"LLM returned invalid JSON"}`), timeout: time.Second, wantErr: ErrUnavailable},
		{name: "500 → unavailable", handler: reply(http.StatusInternalServerError, ""), timeout: time.Second, wantErr: ErrUnavailable},
		{name: "504 → timeout", handler: reply(http.StatusGatewayTimeout, `{"detail":"LLM timed out"}`), timeout: time.Second, wantErr: ErrTimeout},
		{name: "not JSON → unavailable", handler: reply(http.StatusOK, "<html>"), timeout: time.Second, wantErr: ErrUnavailable},
		{name: "data after the JSON object → unavailable", handler: reply(http.StatusOK, okBody+` garbage {"x":1}`), timeout: time.Second, wantErr: ErrUnavailable},
		{name: "body over 1 MB → unavailable", handler: reply(http.StatusOK, okBody+strings.Repeat(" ", maxResponseBytes)), timeout: time.Second, wantErr: ErrUnavailable},
		{name: "empty it_context → unavailable", handler: reply(http.StatusOK, `{"translation":"t","example":"e","it_context":" "}`), timeout: time.Second, wantErr: ErrUnavailable},
		{name: "slow server → timeout", handler: slowHandler, timeout: 50 * time.Millisecond, wantErr: ErrTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			got, err := NewClient(srv.URL, tt.timeout).Enrich(t.Context(), "roll out")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := Enrichment{Translation: "выкатить", Example: "We roll out on Friday.", ITContext: "Release a new version."}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

// The client must send POST /enrich with the phrase as JSON.
func TestClient_Enrich_SendsPhrase(t *testing.T) {
	var gotMethod, gotPath, gotType, gotPhrase string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		var body struct {
			Phrase string `json:"phrase"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotPhrase = body.Phrase
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	// A trailing slash in the base URL must not produce "//enrich".
	if _, err := NewClient(srv.URL+"/", time.Second).Enrich(t.Context(), "roll out"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/enrich" || gotType != "application/json" || gotPhrase != "roll out" {
		t.Errorf("request = %s %s (%s) phrase %q, want POST /enrich (application/json) phrase %q",
			gotMethod, gotPath, gotType, gotPhrase, "roll out")
	}
}

// The caller's deadline must stop the call even if the client timeout is longer:
// this is how the HTTP request context reaches the AI call.
func TestClient_Enrich_UsesCallerContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(slowHandler))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := NewClient(srv.URL, 10*time.Second).Enrich(ctx, "roll out")

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("error = %v, want %v", err, ErrTimeout)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("call took %v, want it to stop at the caller's deadline", elapsed)
	}
}

func TestClient_Enrich_ServerDown(t *testing.T) {
	srv := httptest.NewServer(reply(http.StatusOK, okBody))
	url := srv.URL
	srv.Close() // nothing listens on this address now

	_, err := NewClient(url, time.Second).Enrich(t.Context(), "roll out")

	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want %v", err, ErrUnavailable)
	}
}
