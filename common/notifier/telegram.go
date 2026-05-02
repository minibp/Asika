package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/telebot.v3"
)

// TelegramNotifier sends notifications via Telegram Bot
type TelegramNotifier struct {
	bot *telebot.Bot
	// chatIDs can be user IDs, group IDs, or channel usernames
	chatIDs []string
}

// NewTelegramNotifier creates a new Telegram notifier
func NewTelegramNotifier(config map[string]interface{}) *TelegramNotifier {
	token, _ := config["token"].(string)
	if token == "" {
		slog.Warn("telegram notifier: no token configured")
		return nil
	}

	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		slog.Error("telegram notifier: failed to create bot", "error", err)
		return nil
	}

	chatIDs := make([]string, 0)
	if toList, ok := config["to"].([]interface{}); ok {
		for _, t := range toList {
			if s, ok := t.(string); ok {
				chatIDs = append(chatIDs, s)
			}
		}
	}

	return &TelegramNotifier{
		bot:     bot,
		chatIDs: chatIDs,
	}
}

// Type returns the type of notifier
func (n *TelegramNotifier) Type() string {
	return "telegram"
}

// Bot returns the underlying telebot instance for interactive use
func (n *TelegramNotifier) Bot() *telebot.Bot {
	return n.bot
}

// ChatIDs returns configured chat IDs
func (n *TelegramNotifier) ChatIDs() []string {
	return n.chatIDs
}

// Send sends a notification via Telegram
func (n *TelegramNotifier) Send(ctx context.Context, title, body string) error {
	if len(n.chatIDs) == 0 {
		return fmt.Errorf("no chat IDs configured for telegram notifier")
	}

	message := fmt.Sprintf("*%s*\n\n%s", title, body)

	var lastErr error
	for _, chatID := range n.chatIDs {
		recipient := resolveRecipient(chatID)
		if recipient == nil {
			slog.Warn("telegram notifier: invalid chat ID format", "chat_id", chatID)
			continue
		}

		_, err := n.bot.Send(recipient, message, &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdown,
		})
		if err != nil {
			slog.Error("telegram notifier: failed to send", "chat_id", chatID, "error", err)
			lastErr = err
			continue
		}
		slog.Info("telegram notification sent", "chat_id", chatID)
	}

	if lastErr != nil {
		return fmt.Errorf("telegram send failed for some recipients: %w", lastErr)
	}
	return nil
}

// resolveRecipient converts a string chat ID to a telebot.Recipient
func resolveRecipient(chatID string) telebot.Recipient {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}

	// Channel username starts with @
	if strings.HasPrefix(chatID, "@") {
		return &telebot.Chat{Username: strings.TrimPrefix(chatID, "@")}
	}

	// Numeric chat ID
	id := int64(0)
	if _, err := fmt.Sscanf(chatID, "%d", &id); err == nil && id != 0 {
		return &telebot.Chat{ID: id}
	}

	return nil
}