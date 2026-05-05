package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"time"
)

const (
	dingtalkMaxBodyRead    = 1 << 20 // 1 MB
	dingtalkRequestTimeout = 10 * time.Second
	dingtalkMaxRetries     = 3
)

// DingTalkNotifier sends notifications via DingTalk custom bot webhook.
// Supports text, markdown, link, actionCard, and feedCard message types.
// See: https://open.dingtalk.com/document/orgapp/custom-bot-creation-and-authentication
type DingTalkNotifier struct {
	webhookURL string
	secret     string // optional, for signature
	atMobiles  []string
	atAll      bool
	msgType    string // "text" (default), "markdown", "link", "actionCard", "feedCard"

	httpClient *http.Client
}

// NewDingTalkNotifier creates a new DingTalk notifier from config.
func NewDingTalkNotifier(config map[string]interface{}) Notifier {
	webhookURL, _ := config["webhook_url"].(string)
	if webhookURL == "" {
		slog.Warn("dingtalk notifier: no webhook_url configured")
		return nil
	}

	n := &DingTalkNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: dingtalkRequestTimeout},
		msgType:    "text",
	}

	n.secret, _ = config["secret"].(string)

	if mt, ok := config["msg_type"].(string); ok && mt != "" {
		n.msgType = mt
	}

	if mobiles, ok := config["at_mobiles"].([]interface{}); ok {
		for _, v := range mobiles {
			if s, ok := v.(string); ok && s != "" {
				n.atMobiles = append(n.atMobiles, s)
			}
		}
	}

	if atAll, ok := config["at_all"].(bool); ok {
		n.atAll = atAll
	}

	return n
}

// Type returns the notifier type.
func (n *DingTalkNotifier) Type() string {
	return "dingtalk"
}

// Send sends a notification via DingTalk webhook.
func (n *DingTalkNotifier) Send(ctx context.Context, title, body string) error {
	payload, err := n.buildPayload(title, body)
	if err != nil {
		return fmt.Errorf("dingtalk: build payload failed: %w", err)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("dingtalk: marshal payload failed: %w", err)
	}

	webhookURL := n.signURL(n.webhookURL)

	var lastErr error
	for attempt := 0; attempt <= dingtalkMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			slog.Info("dingtalk webhook retrying", "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := n.doRequest(ctx, webhookURL, data)
		if err == nil {
			slog.Info("dingtalk notification sent")
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("dingtalk webhook failed after %d retries: %w", dingtalkMaxRetries, lastErr)
}

// doRequest performs a single HTTP POST and checks the DingTalk response.
func (n *DingTalkNotifier) doRequest(ctx context.Context, webhookURL string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, dingtalkMaxBodyRead))
		return fmt.Errorf("dingtalk webhook returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, dingtalkMaxBodyRead))
	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if json.Unmarshal(respBody, &result) != nil {
		return fmt.Errorf("dingtalk: failed to parse response: %s", string(respBody))
	}
	if result.Errcode != 0 {
		return fmt.Errorf("dingtalk API error: code=%d msg=%s", result.Errcode, result.Errmsg)
	}

	return nil
}

// buildPayload constructs the DingTalk message payload based on msg_type.
func (n *DingTalkNotifier) buildPayload(title, body string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"msgtype": n.msgType,
	}

	at := map[string]interface{}{
		"atMobiles": n.atMobiles,
		"isAtAll":   n.atAll,
	}
	payload["at"] = at

	switch n.msgType {
	case "markdown":
		payload["markdown"] = map[string]interface{}{
			"title": title,
			"text":  fmt.Sprintf("## %s\n\n%s", title, body),
		}

	case "link":
		payload["link"] = map[string]interface{}{
			"title":      title,
			"text":       body,
			"picURL":     "",
			"messageURL": "", // caller can set via config if needed
		}

	case "actionCard":
		payload["actionCard"] = map[string]interface{}{
			"title":          title,
			"text":           body,
			"btnOrientation": "0",
			"singleTitle":    "View Details",
			"singleURL":      "",
		}

	case "feedCard":
		payload["feedCard"] = map[string]interface{}{
			"links": []map[string]interface{}{
				{
					"title":      title,
					"messageURL": "",
					"picURL":     "",
				},
			},
		}

	default: // "text"
		content := fmt.Sprintf("%s\n\n%s", title, body)
		text := map[string]interface{}{"content": content}
		payload["text"] = text
	}

	return payload, nil
}

// signURL appends HMAC-SHA256 signature to the webhook URL if secret is configured.
func (n *DingTalkNotifier) signURL(baseURL string) string {
	if n.secret == "" {
		return baseURL
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := timestamp + "\n" + n.secret

	mac := hmac.New(sha256.New, []byte(n.secret))
	mac.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s&timestamp=%s&sign=%s", baseURL, timestamp, url.QueryEscape(sign))
}
