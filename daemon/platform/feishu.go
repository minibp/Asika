package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
	"asika/daemon/queue"
	"asika/daemon/syncer"
)

// Bot wraps the Feishu/Lark bot with Asika management functionality.
type FeishuBot struct {
	cfg          *models.Config
	clients      map[platforms.PlatformType]platforms.PlatformClient
	queueMgr     *queue.Manager
	syncerRef    *syncer.Syncer
	spamDetector *syncer.SpamDetector
	notifier     *notifier.FeishuNotifier
	adminIDs     map[string]bool
	stop         chan struct{}
	feishuCfg    models.FeishuConfig
}

// NewBot creates a new Feishu bot.
func NewFeishuBot(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncerRef *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
	n *notifier.FeishuNotifier,
) *FeishuBot {
	b := &FeishuBot{
		cfg:          cfg,
		clients:      clients,
		queueMgr:     queueMgr,
		syncerRef:    syncerRef,
		spamDetector: spamDetector,
		notifier:     n,
		adminIDs:     make(map[string]bool),
		stop:         make(chan struct{}),
		feishuCfg:    cfg.Feishu,
	}
	for _, id := range cfg.Feishu.AdminIDs {
		b.adminIDs[id] = true
	}
	return b
}

// Start starts the bot (sets up HTTP handlers if needed via external routing).
func (b *FeishuBot) Start() {
	slog.Info("starting feishu interactive bot")
}

// Stop stops the bot gracefully.
func (b *FeishuBot) Stop() {
	close(b.stop)
	slog.Info("feishu bot stopped")
}

// HandleEvent handles an incoming Feishu event (called from HTTP handler).
// Returns a response body or nil if no response needed.
func (b *FeishuBot) HandleEvent(ctx context.Context, body []byte) (interface{}, error) {
	var event struct {
		Schema string          `json:"schema"`
		Header struct {
			EventType string `json:"event_type"`
			Token     string `json:"token"`
		} `json:"header"`
		Event json.RawMessage `json:"event"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		slog.Error("feishu: failed to parse event", "error", err)
		return nil, err
	}

	switch event.Header.EventType {
	case "im.message.receive_v1":
		return b.handleMessageEvent(ctx, event.Event)
	case "url_verification":
		return b.handleURLVerification(event.Event)
	default:
		slog.Debug("feishu: unhandled event type", "type", event.Header.EventType)
	}

	return nil, nil
}

// handleURLVerification handles the URL verification challenge.
func (b *FeishuBot) handleURLVerification(raw json.RawMessage) (interface{}, error) {
	var challenge struct {
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return nil, err
	}

	return map[string]string{
		"challenge": challenge.Challenge,
	}, nil
}

// handleMessageEvent handles incoming messages (commands).
func (b *FeishuBot) handleMessageEvent(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	var msg struct {
		Sender struct {
			SenderID struct {
				UserID string `json:"user_id"`
			} `json:"sender_id"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			Content     string `json:"content"`
			MessageType string `json:"message_type"`
		} `json:"message"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message event: %w", err)
	}

	senderID := msg.Sender.SenderID.UserID
	chatID := msg.Message.ChatID
	contentStr := msg.Message.Content

	// Parse text content from Feishu message JSON
	text := b.parseMessageText(contentStr)
	if text == "" {
		return nil, nil
	}

	slog.Info("feishu bot: received message", "sender", senderID, "chat", chatID, "text", text)

	// Build reply message text
	reply := b.processCommand(senderID, text)

	if reply != "" {
		return map[string]interface{}{
			"msg_type": "text",
			"content": map[string]interface{}{
				"text": reply,
			},
		}, nil
	}

	return nil, nil
}

// parseMessageText extracts text content from Feishu message JSON.
// Feishu wraps message content in a JSON string like {"text":"hello"}.
func (b *FeishuBot) parseMessageText(contentStr string) string {
	if contentStr == "" {
		return ""
	}

	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(contentStr), &content); err != nil {
		// Try raw text
		return strings.TrimSpace(contentStr)
	}
	return strings.TrimSpace(content.Text)
}

// processCommand processes a text command and returns a reply string.
func (b *FeishuBot) processCommand(senderID, text string) string {
	if !b.isAdmin(senderID) {
		return "Access denied. Admin only."
	}

	lower := strings.ToLower(text)
	parts := strings.Fields(text)

	switch {
	case lower == "help" || lower == "/help":
		return b.helpText()

	case lower == "prs" || lower == "/prs":
		return b.listPRsText("")

	case strings.HasPrefix(lower, "prs ") || strings.HasPrefix(lower, "/prs "):
		groupName := ""
		if len(parts) > 1 {
			groupName = parts[1]
		}
		return b.listPRsText(groupName)

	case strings.HasPrefix(lower, "pr ") || strings.HasPrefix(lower, "/pr "):
		if len(parts) < 3 {
			return "Usage: pr <repo_group> <pr_number>"
		}
		return b.showPRText(parts[1], parts[2])

	case strings.HasPrefix(lower, "approve ") || strings.HasPrefix(lower, "/approve "):
		if len(parts) < 3 {
			return "Usage: approve <repo_group> <pr_id>"
		}
		return b.doApprove(senderID, parts[1], parts[2])

	case strings.HasPrefix(lower, "close ") || strings.HasPrefix(lower, "/close "):
		if len(parts) < 3 {
			return "Usage: close <repo_group> <pr_id>"
		}
		return b.doClose(senderID, parts[1], parts[2])

	case strings.HasPrefix(lower, "reopen ") || strings.HasPrefix(lower, "/reopen "):
		if len(parts) < 3 {
			return "Usage: reopen <repo_group> <pr_id>"
		}
		return b.doReopen(senderID, parts[1], parts[2])

	case strings.HasPrefix(lower, "spam ") || strings.HasPrefix(lower, "/spam "):
		if len(parts) < 3 {
			return "Usage: spam <repo_group> <pr_id>"
		}
		return b.doMarkSpam(senderID, parts[1], parts[2])

	case lower == "queue" || lower == "/queue":
		return b.showQueueText("")

	case strings.HasPrefix(lower, "queue ") || strings.HasPrefix(lower, "/queue "):
		groupName := ""
		if len(parts) > 1 {
			groupName = parts[1]
		}
		return b.showQueueText(groupName)

	case lower == "recheck" || lower == "/recheck":
		if b.queueMgr != nil {
			go b.queueMgr.CheckQueue()
			return "Queue recheck triggered."
		}
		return "Queue manager not initialized."

	case lower == "config" || lower == "/config":
		return b.showConfigText()

	default:
		return fmt.Sprintf("Unknown command: %s\nTry 'help' for available commands.", text)
	}
}

func (b *FeishuBot) helpText() string {
	return `Asika Feishu Bot Commands:
  help          - Show this help
  prs [group]   - List PRs
  pr <group> <num> - Show PR details
  approve <group> <id> - Approve PR
  close <group> <id>   - Close PR
  reopen <group> <id>  - Reopen PR
  spam <group> <id>    - Mark as spam
  queue [group] - Show merge queue
  recheck       - Trigger queue recheck
  config        - Show config summary`
}

func (b *FeishuBot) listPRsText(repoGroup string) string {
	if repoGroup == "" {
		groups := config.GetRepoGroups(b.cfg)
		if len(groups) == 0 {
			return "No repo groups configured."
		}
		repoGroup = groups[0].Name
	}

	var prs []models.PRRecord
	db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if err := json.Unmarshal(value, &pr); err != nil {
			return nil
		}
		if pr.RepoGroup == repoGroup || repoGroup == "" {
			prs = append(prs, pr)
		}
		return nil
	})

	if len(prs) == 0 {
		return fmt.Sprintf("No PRs in %s", repoGroup)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PRs in %s:\n", repoGroup))
	for _, pr := range prs {
		emoji := "O"
		switch pr.State {
		case "merged":
			emoji = "M"
		case "closed":
			emoji = "X"
		case "spam":
			emoji = "!"
		}
		sb.WriteString(fmt.Sprintf("  %s #%d %s - %s (%s)\n",
			emoji, pr.PRNumber, truncateStr(pr.Title, 35), pr.Author, pr.State))
	}
	return sb.String()
}

func (b *FeishuBot) showPRText(repoGroup, prID string) string {
	pr, _ := getPRRecord(repoGroup, prID)
	if pr == nil {
		return fmt.Sprintf("PR %s not found in %s", prID, repoGroup)
	}
	return fmt.Sprintf(
		"PR #%d - %s\n  Author: %s | State: %s\n  Platform: %s | Spam: %v\n  Labels: %s",
		pr.PRNumber, pr.Title, pr.Author, pr.State,
		pr.Platform, pr.SpamFlag, strings.Join(pr.Labels, ", "),
	)
}

func (b *FeishuBot) doApprove(senderID, repoGroup, prID string) string {
	pr, _ := getPRRecord(repoGroup, prID)
	if pr == nil {
		return "PR not found."
	}
	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return "Repo group not found."
	}
	client := b.getClient(pr.Platform)
	if client == nil {
		return "No client for platform."
	}
	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if err := client.ApprovePR(context.Background(), owner, repo, pr.PRNumber); err != nil {
		return fmt.Sprintf("Failed: %v", err)
	}
	return fmt.Sprintf("PR #%d approved.", pr.PRNumber)
}

func (b *FeishuBot) doClose(senderID, repoGroup, prID string) string {
	pr, _ := getPRRecord(repoGroup, prID)
	if pr == nil {
		return "PR not found."
	}
	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return "Repo group not found."
	}
	client := b.getClient(pr.Platform)
	if client == nil {
		return "No client for platform."
	}
	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if err := client.ClosePR(context.Background(), owner, repo, pr.PRNumber); err != nil {
		return fmt.Sprintf("Failed: %v", err)
	}
	return fmt.Sprintf("PR #%d closed.", pr.PRNumber)
}

func (b *FeishuBot) doReopen(senderID, repoGroup, prID string) string {
	pr, _ := getPRRecord(repoGroup, prID)
	if pr == nil {
		return "PR not found."
	}
	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return "Repo group not found."
	}
	client := b.getClient(pr.Platform)
	if client == nil {
		return "No client for platform."
	}
	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if err := client.ReopenPR(context.Background(), owner, repo, pr.PRNumber); err != nil {
		return fmt.Sprintf("Failed: %v", err)
	}
	return fmt.Sprintf("PR #%d reopened.", pr.PRNumber)
}

func (b *FeishuBot) doMarkSpam(senderID, repoGroup, prID string) string {
	pr, _ := getPRRecord(repoGroup, prID)
	if pr == nil {
		return "PR not found."
	}
	pr.SpamFlag = true
	pr.State = "spam"
	pr.UpdatedAt = time.Now()
	key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, key, data)

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group != nil {
		client := b.getClient(pr.Platform)
		if client != nil {
			owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
			client.ClosePR(context.Background(), owner, repo, pr.PRNumber)
		}
	}

	if b.notifier != nil {
		title := fmt.Sprintf("[Spam Alert] PR #%d", pr.PRNumber)
		body := fmt.Sprintf("PR #%d \"%s\" by %s marked as spam via Feishu.", pr.PRNumber, pr.Title, pr.Author)
		b.notifier.Send(context.Background(), title, body)
	}

	return fmt.Sprintf("PR #%d marked as spam.", pr.PRNumber)
}

func (b *FeishuBot) showQueueText(repoGroup string) string {
	if repoGroup == "" {
		groups := config.GetRepoGroups(b.cfg)
		if len(groups) > 0 {
			repoGroup = groups[0].Name
		}
	}

	var items []models.QueueItem
	db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
		var item models.QueueItem
		if err := json.Unmarshal(value, &item); err != nil {
			return nil
		}
		if strings.HasPrefix(string(key), repoGroup+"#") {
			items = append(items, item)
		}
		return nil
	})

	if len(items) == 0 {
		return fmt.Sprintf("Queue empty for %s", repoGroup)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Queue - %s:\n", repoGroup))
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("  %s (%s)\n", item.PRID, item.Status))
	}
	return sb.String()
}

func (b *FeishuBot) showConfigText() string {
	cfg := config.Current()
	if cfg == nil {
		return "Config not loaded."
	}
	groups := config.GetRepoGroups(cfg)
	return fmt.Sprintf(
		"Asika Config:\n  Server: %s (%s)\n  DB: %s\n  Events: %s\n  Spam: %v\n  Repo Groups: %d\n  Notify Channels: %d",
		cfg.Server.Listen, cfg.Server.Mode, cfg.Database.Path,
		cfg.Events.Mode, cfg.Spam.Enabled, len(groups), len(cfg.Notify),
	)
}

func (b *FeishuBot) isAdmin(userID string) bool {
	if len(b.adminIDs) == 0 {
		return true
	}
	return b.adminIDs[userID]
}

func (b *FeishuBot) getClient(platform string) platforms.PlatformClient {
	if b.clients == nil {
		return nil
	}
	return b.clients[platforms.PlatformType(platform)]
}

// getPRRecord finds a PR by repo group and ID or number.
func getPRRecord(repoGroup, idOrNumber string) (*models.PRRecord, error) {
	var found *models.PRRecord
	prNumber, _ := strconv.Atoi(idOrNumber)

	db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if err := json.Unmarshal(value, &pr); err != nil {
			return nil
		}
		if pr.RepoGroup == repoGroup {
			if pr.ID == idOrNumber || (prNumber > 0 && pr.PRNumber == prNumber) {
				found = &pr
			}
		}
		return nil
	})

	if found == nil {
		return nil, fmt.Errorf("PR not found")
	}
	return found, nil
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}