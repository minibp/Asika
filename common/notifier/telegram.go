package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"gopkg.in/telebot.v3"
)

// TelegramNotifier sends notifications via Telegram Bot
type TelegramNotifier struct {
	bot        *telebot.Bot
	chatIDs    []string
	httpToken  string
	httpAPIURL string
}

// NewTelegramNotifier creates a new Telegram notifier
func NewTelegramNotifier(config map[string]interface{}) *TelegramNotifier {
	token, _ := config["token"].(string)
	if token == "" {
		slog.Warn("telegram notifier: no token configured")
		return nil
	}

	// Check if http mode is requested (webhook_url or explicit http_token)
	httpToken := ""
	if webhookURL, ok := config["webhook_url"].(string); ok && webhookURL != "" {
		// Extract token from webhook URL or use the main token
		httpToken = token
	}
	if t, ok := config["http_token"].(string); ok && t != "" {
		httpToken = t
	}

	n := &TelegramNotifier{
		httpToken:  httpToken,
		httpAPIURL: "https://api.telegram.org",
	}

	// Init SDK bot only if not using HTTP mode
	if httpToken == "" {
		pref := telebot.Settings{
			Token:  token,
			Poller: &telebot.LongPoller{Timeout: 10},
		}

		bot, err := telebot.NewBot(pref)
		if err != nil {
			slog.Error("telegram notifier: failed to create bot", "error", err)
			return nil
		}
		n.bot = bot
	}

	chatIDs := make([]string, 0)
	if toList, ok := config["to"].([]interface{}); ok {
		for _, t := range toList {
			if s, ok := t.(string); ok {
				chatIDs = append(chatIDs, s)
			}
		}
	}

	n.chatIDs = chatIDs
	return n
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

	// HTTP mode: direct API call (no SDK)
	if n.httpToken != "" {
		return n.sendViaHTTP(ctx, title, body)
	}

	// SDK mode: use telebot (requires bot instance)
	if n.bot == nil {
		return fmt.Errorf("telegram: bot not initialized")
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

// sendViaHTTP sends notification via Telegram HTTP API (no SDK needed)
func (n *TelegramNotifier) sendViaHTTP(ctx context.Context, title, body string) error {
	message := fmt.Sprintf("*%s*\n\n%s", title, body)

	var lastErr error
	for _, chatID := range n.chatIDs {
		apiURL := fmt.Sprintf("%s/bot%s/sendMessage", n.httpAPIURL, n.httpToken)

		payload := map[string]interface{}{
			"chat_id":    chatID,
			"text":       message,
			"parse_mode": "Markdown",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			lastErr = err
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(data))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error("telegram http: send failed", "chat_id", chatID, "error", err)
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("telegram http: send failed", "chat_id", chatID, "status", resp.StatusCode)
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		slog.Info("telegram notification sent via http", "chat_id", chatID)
	}

	if lastErr != nil {
		return fmt.Errorf("telegram http send failed for some recipients: %w", lastErr)
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