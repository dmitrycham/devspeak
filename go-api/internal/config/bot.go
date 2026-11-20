package config

import (
	"fmt"
	"strconv"
)

// BotConfig holds the Telegram bot settings.
type BotConfig struct {
	Token         string // bot token from @BotFather
	AllowedUserID int64  // the only Telegram user who can use the bot
}

// LoadBot reads the bot settings from environment variables.
// TELEGRAM_BOT_TOKEN and TELEGRAM_ALLOWED_USER_ID are required.
func LoadBot() (BotConfig, error) {
	token, err := required("TELEGRAM_BOT_TOKEN")
	if err != nil {
		return BotConfig{}, err
	}
	rawID, err := required("TELEGRAM_ALLOWED_USER_ID")
	if err != nil {
		return BotConfig{}, err
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return BotConfig{}, fmt.Errorf("env TELEGRAM_ALLOWED_USER_ID must be a number: %w", err)
	}

	return BotConfig{Token: token, AllowedUserID: id}, nil
}
