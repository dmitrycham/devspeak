package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/dmitrycham/devspeak/go-api/internal/cards"
	"github.com/dmitrycham/devspeak/go-api/internal/srs"
)

// CardService is what the bot needs from the service layer.
// It is declared here, by the consumer, so tests can pass a fake.
type CardService interface {
	Create(ctx context.Context, in cards.CreateInput) (cards.Card, error)
	Get(ctx context.Context, id int64) (cards.Card, error)
	NextDue(ctx context.Context) (cards.Card, error)
	Review(ctx context.Context, id int64, grade srs.Grade) (cards.Card, error)
}

// Handler answers Telegram updates.
type Handler struct {
	svc           CardService
	allowedUserID int64
	logger        *slog.Logger
}

// NewHandler creates a Handler that serves only the user allowedUserID.
func NewHandler(svc CardService, allowedUserID int64, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, allowedUserID: allowedUserID, logger: logger}
}

// Run connects to Telegram with long polling and blocks until ctx is cancelled.
// Long polling means the bot asks Telegram for updates, so no public server is needed.
func (h *Handler) Run(ctx context.Context, token string) error {
	// The library checks the token with getMe and waits only 5 seconds by default.
	// That is too short on a slow network or through a VPN.
	b, err := tg.New(token,
		tg.WithMiddlewares(h.onlyAllowed),
		tg.WithCheckInitTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	b.RegisterHandler(tg.HandlerTypeMessageText, "/add", tg.MatchTypePrefix, h.add)
	b.RegisterHandler(tg.HandlerTypeMessageText, "/review", tg.MatchTypeExact, h.review)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, showPrefix, tg.MatchTypePrefix, h.show)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, gradePrefix, tg.MatchTypePrefix, h.grade)

	h.logger.Info("bot started")
	b.Start(ctx)
	return nil
}

// onlyAllowed drops every update that is not from the allowed user.
// We stay silent on purpose: a stranger gets no hint that the bot works.
func (h *Handler) onlyAllowed(next tg.HandlerFunc) tg.HandlerFunc {
	return func(ctx context.Context, b *tg.Bot, update *models.Update) {
		if senderID(update) != h.allowedUserID {
			return
		}
		next(ctx, b, update)
	}
}

// senderID returns the Telegram user id of the update, or 0 if unknown.
func senderID(update *models.Update) int64 {
	switch {
	case update.Message != nil && update.Message.From != nil:
		return update.Message.From.ID
	case update.CallbackQuery != nil:
		return update.CallbackQuery.From.ID
	default:
		return 0
	}
}

// add handles "/add phrase — translation". One message may hold several /add lines.
func (h *Handler) add(ctx context.Context, b *tg.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	inputs, err := ParseAddAll(update.Message.Text)
	if err != nil {
		h.send(ctx, b, chatID, err.Error(), nil)
		return
	}

	var saved int
	var duplicates []string
	for _, in := range inputs {
		_, err := h.svc.Create(ctx, in)
		switch {
		case errors.Is(err, cards.ErrDuplicate):
			// A duplicate must not stop the rest of the pasted list.
			duplicates = append(duplicates, in.Phrase)
		case err != nil:
			h.logger.Error("create card", "error", err)
			h.send(ctx, b, chatID, fmt.Sprintf("Something went wrong. Saved before the error: %d.", saved), nil)
			return
		default:
			saved++
		}
	}

	h.send(ctx, b, chatID, addReply(saved, duplicates), nil)
}

// review handles "/review": it shows the next due card.
func (h *Handler) review(ctx context.Context, b *tg.Bot, update *models.Update) {
	h.sendNextCard(ctx, b, update.Message.Chat.ID)
}

// show handles the "Show" button: it reveals the English phrase
// and asks for a grade.
func (h *Handler) show(ctx context.Context, b *tg.Bot, update *models.Update) {
	h.ack(ctx, b, update)

	msg := update.CallbackQuery.Message.Message
	if msg == nil {
		return // Telegram no longer has the message; nothing to edit
	}

	id, err := parseShow(update.CallbackQuery.Data)
	if err != nil {
		h.logger.Warn("parse show callback", "error", err)
		return
	}

	card, err := h.svc.Get(ctx, id)
	if err != nil {
		h.logger.Error("get card", "id", id, "error", err)
		h.send(ctx, b, msg.Chat.ID, "Card not found.", nil)
		return
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "✅ Knew", CallbackData: gradeData(card.ID, srs.Knew)},
			{Text: "❌ Forgot", CallbackData: gradeData(card.ID, srs.Forgot)},
		}},
	}
	_, err = b.EditMessageText(ctx, &tg.EditMessageTextParams{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.ID,
		Text:        card.Translation + "\n\n" + card.Phrase,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		h.logger.Error("edit message", "error", err)
	}
}

// grade handles the "Knew" and "Forgot" buttons: it saves the grade
// and sends the next card.
func (h *Handler) grade(ctx context.Context, b *tg.Bot, update *models.Update) {
	h.ack(ctx, b, update)

	msg := update.CallbackQuery.Message.Message
	if msg == nil {
		return
	}

	id, grade, err := parseGrade(update.CallbackQuery.Data)
	if err != nil {
		h.logger.Warn("parse grade callback", "error", err)
		return
	}

	if _, err := h.svc.Review(ctx, id, grade); err != nil {
		h.logger.Error("review card", "id", id, "error", err)
		h.send(ctx, b, msg.Chat.ID, "Could not save the grade. Try again.", nil)
		return
	}

	h.sendNextCard(ctx, b, msg.Chat.ID)
}

// sendNextCard sends the Russian side of the next due card with a "Show" button,
// or tells the user that nothing is due.
func (h *Handler) sendNextCard(ctx context.Context, b *tg.Bot, chatID int64) {
	card, err := h.svc.NextDue(ctx)
	switch {
	case errors.Is(err, cards.ErrNotFound):
		h.send(ctx, b, chatID, "All done for today", nil)
		return
	case err != nil:
		h.logger.Error("next due card", "error", err)
		h.send(ctx, b, chatID, "Something went wrong. Try again later.", nil)
		return
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "Show", CallbackData: showData(card.ID)},
		}},
	}
	h.send(ctx, b, chatID, card.Translation, keyboard)
}

// ack tells Telegram that the button press was received.
// Without it the button shows a loading spinner.
func (h *Handler) ack(ctx context.Context, b *tg.Bot, update *models.Update) {
	_, err := b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	if err != nil {
		h.logger.Warn("answer callback query", "error", err)
	}
}

// send sends a text message and logs the error, if any.
func (h *Handler) send(ctx context.Context, b *tg.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := b.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: markup,
	})
	if err != nil {
		h.logger.Error("send message", "error", err)
	}
}
