package notifier

import (
    "context"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "strings"
)

// WeComNotifier sends notifications via WeCom webhook
type WeComNotifier struct {
    webhookURL string
}

// NewWeComNotifier creates a new WeCom notifier
func NewWeComNotifier(config map[string]interface{}) *WeComNotifier {
    webhookURL, _ := config["webhook_url"].(string)
    return &WeComNotifier{
        webhookURL: webhookURL,
    }
}

// Type returns the type of notifier
func (n *WeComNotifier) Type() string {
    return "wecom"
}

// Send sends a notification
func (n *WeComNotifier) Send(ctx context.Context, title, body string) error {
    if n.webhookURL == "" {
        return fmt.Errorf("webhook URL not configured")
    }

    content := fmt.Sprintf("**%s**\n\n%s", title, body)
    payload := fmt.Sprintf(`{"msgtype":"markdown","markdown":{"content":"%s"}}`, escapeJSON(content))

    req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, strings.NewReader(payload))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        slog.Error("wecom webhook failed", "status", resp.StatusCode, "body", string(bodyBytes))
        return fmt.Errorf("wecom webhook returned status %d", resp.StatusCode)
    }

    slog.Info("wecom notification sent")
    return nil
}

// escapeJSON escapes a string for JSON
func escapeJSON(s string) string {
    s = strings.ReplaceAll(s, "\\", "\\\\")
    s = strings.ReplaceAll(s, "\"", "\\\"")
    s = strings.ReplaceAll(s, "\n", "\\n")
    return s
}
