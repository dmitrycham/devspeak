package cards

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// maxBodyBytes limits the request body size (1 MB).
const maxBodyBytes = 1 << 20

// cardService is what the handler needs from the business layer.
// It is declared here, by the consumer, so tests can pass a fake.
// *Service implements it.
type cardService interface {
	Create(ctx context.Context, in CreateInput) (Card, error)
	List(ctx context.Context) ([]Card, error)
	Get(ctx context.Context, id int64) (Card, error)
	Delete(ctx context.Context, id int64) error
	NextDue(ctx context.Context) (Card, error)
	Review(ctx context.Context, id int64, grade srs.Grade) (Card, error)
}

// Handler serves the cards and reviews HTTP API.
type Handler struct {
	svc    cardService
	logger *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc cardService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes returns the HTTP routes (Go 1.22+ patterns: "METHOD /path/{param}").
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cards", h.create)
	mux.HandleFunc("GET /cards", h.list)
	mux.HandleFunc("GET /cards/{id}", h.get)
	mux.HandleFunc("DELETE /cards/{id}", h.delete)
	mux.HandleFunc("GET /reviews/next", h.nextReview)
	mux.HandleFunc("POST /reviews/{id}", h.review)
	return mux
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !decodeJSON(w, r, &in) {
		return
	}
	card, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	cards, err := h.svc.List(r.Context())
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cards)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	card, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
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

// nextReview returns the next due card, or 204 No Content if nothing is due.
func (h *Handler) nextReview(w http.ResponseWriter, r *http.Request) {
	card, err := h.svc.NextDue(r.Context())
	if errors.Is(err, ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

// reviewRequest is the body of POST /reviews/{id}.
type reviewRequest struct {
	Grade string `json:"grade"`
}

// review applies a grade ("knew" or "forgot") and returns the updated card.
func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req reviewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	grade, err := srs.ParseGrade(req.Grade)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	card, err := h.svc.Review(r.Context(), id, grade)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

// decodeJSON reads the request body into v. It writes 400 and returns false
// if the body is not valid JSON, has unknown fields or is bigger than 1 MB.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
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
