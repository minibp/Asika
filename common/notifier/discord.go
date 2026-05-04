package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// DiscordNotifier sends notifications via Discord Bot
type DiscordNotifier struct {
	session    *discordgo.Session
	channelIDs []string
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier(config map[string]interface{}) *DiscordNotifier {
	token, _ := config["token"].(string)
	if token == "" {
		slog.Warn("discord notifier: no token configured")
		return nil
	}

	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}

	session, err := discordgo.New(token)
	if err != nil {
		slog.Error("discord notifier: failed to create session", "error", err)
		return nil
	}

	channelIDs := make([]string, 0)
	if toList, ok := config["channel_ids"].([]interface{}); ok {
		for _, t := range toList {
			if s, ok := t.(string); ok && s != "" {
				channelIDs = append(channelIDs, s)
			}
		}
	}

	n := &DiscordNotifier{
		session:    session,
		channelIDs: channelIDs,
	}

	if err := n.session.Open(); err != nil {
		slog.Error("discord notifier: failed to open connection", "error", err)
		return nil
	}

	return n
}

// Type returns the type of notifier
func (n *DiscordNotifier) Type() string {
	return "discord"
}

// Session returns the Discord session
func (n *DiscordNotifier) Session() *discordgo.Session {
	return n.session
}

// Send sends a notification via Discord
func (n *DiscordNotifier) Send(ctx context.Context, title, body string) error {
	if n.session == nil {
		return fmt.Errorf("discord: session not initialized")
	}
	if len(n.channelIDs) == 0 {
		return fmt.Errorf("no channel IDs configured for discord notifier")
	}

	message := fmt.Sprintf("**%s**\n\n%s", title, body)

	var lastErr error
	for _, channelID := range n.channelIDs {
		_, err := n.session.ChannelMessageSend(channelID, message)
		if err != nil {
			slog.Error("discord notifier: failed to send", "channel_id", channelID, "error", err)
			lastErr = err
			continue
		}
		slog.Info("discord notification sent", "channel_id", channelID)
	}

	if lastErr != nil {
		return fmt.Errorf("discord send failed for some channels: %w", lastErr)
	}
	return nil
}

// Close closes the Discord session
func (n *DiscordNotifier) Close() {
	if n.session != nil {
		n.session.Close()
	}
}
