package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// FeishuNotifier sends notifications via Feishu/Lark
type FeishuNotifier struct {
	webhookURL string
	appID     string
	appSecret string
}

// NewFeishuNotifier creates a new Feishu notifier.
// Supports two modes:
//  1. Webhook mode: config["webhook_url"] is set (simpler, no app credentials)
//  2. App mode: config["app_id"] + config["app_secret"] are set (full SDK)
func NewFeishuNotifier(config map[string]interface{}) *FeishuNotifier {
	n := &FeishuNotifier{}

	webhookURL, _ := config["webhook_url"].(string)
	appID, _ := config["app_id"].(string)
	appSecret, _ := config["app_secret"].(string)

	n.webhookURL = webhookURL

	if appID != "" && appSecret != "" {
		n.appID = appID
		n.appSecret = appSecret
	}

	return n
}

// Type returns the type of notifier
func (n *FeishuNotifier) Type() string {
	return "feishu"
}

// WebhookURL returns the configured webhook URL.
func (n *FeishuNotifier) WebhookURL() string {
	return n.webhookURL
}

// Send sends a notification via Feishu webhook or IM API.
func (n *FeishuNotifier) Send(ctx context.Context, title, body string) error {
	if n.webhookURL != "" {
		return n.sendViaWebhook(ctx, title, body)
	}

	if n.appID != "" && n.appSecret != "" {
		return n.sendViaApp(ctx, title, body)
	}

	return fmt.Errorf("feishu: no webhook_url or app credentials configured")
}

// sendViaWebhook sends via simple webhook (like WeCom)
func (n *FeishuNotifier) sendViaWebhook(ctx context.Context, title, body string) error {
	content := formatFeishuCard(title, body)

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card":     content,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("feishu: marshal payload failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("feishu webhook failed", "status", resp.StatusCode, "body", string(bodyBytes))
		return fmt.Errorf("feishu webhook returned status %d", resp.StatusCode)
	}

	slog.Info("feishu notification sent via webhook")
	return nil
}

// sendViaApp sends via Feishu IM API (requires app credentials)
func (n *FeishuNotifier) sendViaApp(ctx context.Context, title, body string) error {
	// This would use the full SDK to send messages via the IM API
	// For now, we log it since the SDK needs a proper client setup
	slog.Warn("feishu: app mode not fully implemented, use webhook mode instead",
		"title", title)
	return fmt.Errorf("feishu: app mode send not implemented, use webhook mode")
}

// formatFeishuCard builds a simple Feishu card message.
func formatFeishuCard(title, body string) map[string]interface{} {
	return map[string]interface{}{
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": title,
			},
			"template": "blue",
		},
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": body,
				},
			},
		},
	}
}

// feishuCardPayload is used for constructing interactive card messages.
// Exported for use by the bot package.
func FeishuCardPayload(title, body string) map[string]interface{} {
	return formatFeishuCard(title, body)
}

func escapeFeishu(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}