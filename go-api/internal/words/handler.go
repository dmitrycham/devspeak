package words

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

// maxBodyBytes limits the request body size (1 MB).
const maxBodyBytes = 1 << 20

// wordService is what the handler needs from the business layer.
// It is declared here, by the consumer, so tests can pass a fake.
// *Service implements it.
type wordService interface {
	Create(ctx context.Context, in CreateInput) (Word, error)
	List(ctx context.Context) ([]Word, error)
	Get(ctx context.Context, id int64) (Word, error)
	Delete(ctx context.Context, id int64) error
}

// Handler serves the words HTTP API.
type Handler struct {
	svc    wordService
	logger *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc wordService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes returns the HTTP routes (Go 1.22+ patterns: "METHOD /path/{param}").
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /words", h.create)
	mux.HandleFunc("GET /words", h.list)
	mux.HandleFunc("GET /words/{id}", h.get)
	mux.HandleFunc("DELETE /words/{id}", h.delete)
	return mux
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var in CreateInput
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	word, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, word)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	words, err := h.svc.List(r.Context())
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, words)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	word, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, word)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseID reads {id} from the path. It writes 400 and returns false
// if the id is not a positive number.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive number")
		return 0, false
	}
	return id, true
}

// writeServiceError maps service errors to HTTP status codes.
// Unknown errors are logged and hidden from the client.
func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrDuplicate):
		writeError(w, http.StatusConflict, err.Error())
	default:
		h.logger.Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status is already sent, so an encode error can't change the response.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
