package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
	"asika/daemon/queue"
	"asika/daemon/syncer"
)

// Bot wraps the Telegram bot with Asika management functionality.
type TelegramBot struct {
	bot          *telebot.Bot
	cfg          *models.Config
	clients      map[platforms.PlatformType]platforms.PlatformClient
	queueMgr     *queue.Manager
	syncerRef    *syncer.Syncer
	spamDetector *syncer.SpamDetector
	notifier     *notifier.TelegramNotifier
	adminIDs     map[int64]bool
	stop         chan struct{}
}

// NewBot creates a new Telegram bot with interactive decision support.
// If telegramNotifier is nil, creates a standalone bot for interactive use only.
func NewTelegramBot(
	bot *telebot.Bot,
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncerRef *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
	telegramNotifier *notifier.TelegramNotifier,
	adminIDs []int64,
) *TelegramBot {
	b := &TelegramBot{
		bot:          bot,
		cfg:          cfg,
		clients:      clients,
		queueMgr:     queueMgr,
		syncerRef:    syncerRef,
		spamDetector: spamDetector,
		notifier:     telegramNotifier,
		adminIDs:     make(map[int64]bool),
		stop:         make(chan struct{}),
	}
	for _, id := range adminIDs {
		b.adminIDs[id] = true
	}
	return b
}

// Start starts the bot polling and registers command handlers.
func (b *TelegramBot) Start() {
	if b.bot == nil {
		slog.Warn("telegram bot: no bot instance, skipping start")
		return
	}

	slog.Info("starting telegram interactive bot")

	b.registerCommands()

	go func() {
		b.bot.Start()
	}()
}

// Stop stops the bot gracefully.
func (b *TelegramBot) Stop() {
	close(b.stop)
	if b.bot != nil {
		b.bot.Stop()
	}
	slog.Info("telegram bot stopped")
}

// registerCommands registers all bot command handlers.
func (b *TelegramBot) registerCommands() {
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/prs", b.handleListPRs)
	b.bot.Handle("/pr", b.handleShowPR)
	b.bot.Handle("/approve", b.handleApprovePR)
	b.bot.Handle("/close", b.handleClosePR)
	b.bot.Handle("/reopen", b.handleReopenPR)
	b.bot.Handle("/spam", b.handleMarkSpam)
	b.bot.Handle("/queue", b.handleShowQueue)
	b.bot.Handle("/recheck", b.handleRecheckQueue)
	b.bot.Handle("/config", b.handleShowConfig)
	b.bot.Handle("/stalecheck", b.handleStaleCheck)
	b.bot.Handle("/unstale", b.handleUnstale)

	// Handle button callbacks for inline decisions
	b.bot.Handle(telebot.OnCallback, b.handleCallback)

	// Handle text messages for natural language-ish commands
	b.bot.Handle(telebot.OnText, b.handleText)
}

// isAdmin checks if the sender is an authorized admin.
func (b *TelegramBot) isAdmin(c telebot.Context) bool {
	if len(b.adminIDs) == 0 {
		return true
	}
	return b.adminIDs[c.Sender().ID]
}

func (b *TelegramBot) requireAdmin(c telebot.Context) bool {
	if !b.isAdmin(c) {
		c.Send("Access denied. This bot is for authorized admins only.")
		return false
	}
	return true
}

// handleStart handles /start command.
func (b *TelegramBot) handleStart(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	userID := c.Sender().ID
	username := c.Sender().Username

	msg := fmt.Sprintf(
		"<b>Welcome to Asika Bot</b>\n\nHello @%s (ID: %d)\n\nUse /help to see available commands.\n\nYou have admin privileges.",
		html.EscapeString(username), userID,
	)

	return c.Send(msg, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

// handleHelp handles /help command.
func (b *TelegramBot) handleHelp(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	help := `<b>Asika Bot Commands</b>

📋 <b>PR Management</b>
/prs repo_group — List PRs
/pr repo_group number — Show PR details
/approve repo_group pr_id — Approve a PR
/close repo_group pr_id — Close a PR
/reopen repo_group pr_id — Reopen a PR (spam recovery)
/spam repo_group pr_id — Mark PR as spam

📊 <b>Queue</b>
/queue repo_group — Show merge queue
/recheck repo_group — Trigger queue recheck

⚙️ <b>Config</b>
/config — Show current config (masked)

🧹 <b>Stale PRs</b>
/stale repo_group — Show stale PRs
/unstale repo_group pr_number — Remove stale label`

	return c.Send(help, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

// handleListPRs handles /prs command.
func (b *TelegramBot) handleListPRs(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	repoGroup := ""
	if len(args) > 1 {
		repoGroup = args[1]
	} else {
		groups := config.GetRepoGroups(b.cfg)
		if len(groups) == 0 {
			return c.Send("No repo groups configured.")
		}
		repoGroup = groups[0].Name
	}

	// Fetch PRs from DB
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
		return c.Send(fmt.Sprintf("No PRs found for repo group <b>%s</b>.", html.EscapeString(repoGroup)),
			&telebot.SendOptions{ParseMode: telebot.ModeHTML})
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>PRs in %s</b>\n\n", html.EscapeString(repoGroup)))
	for _, pr := range prs {
		statusEmoji := "🔵"
		switch pr.State {
		case "merged":
			statusEmoji = "🟣"
		case "closed":
			statusEmoji = "🔴"
		case "spam":
			statusEmoji = "⚠️"
		}
		sb.WriteString(fmt.Sprintf("%s <b>#%d</b> %s — by %s (%s/%s)\n",
			statusEmoji, pr.PRNumber, html.EscapeString(truncate(pr.Title, 40)), html.EscapeString(pr.Author), pr.Platform, pr.State))
	}

	return c.Send(sb.String(), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

// handleShowPR handles /pr command.
func (b *TelegramBot) handleShowPR(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	if len(args) < 3 {
		return c.Send("Usage: /pr repo_group pr_number")
	}

	repoGroup := args[1]
	prNumber, err := strconv.Atoi(args[2])
	if err != nil {
		return c.Send("Invalid PR number.")
	}

	var found *models.PRRecord
	db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if err := json.Unmarshal(value, &pr); err != nil {
			return nil
		}
		if pr.RepoGroup == repoGroup && pr.PRNumber == prNumber {
			found = &pr
		}
		return nil
	})

	if found == nil {
		return c.Send(fmt.Sprintf("PR #%d not found in repo group <b>%s</b>.", prNumber, html.EscapeString(repoGroup)),
			&telebot.SendOptions{ParseMode: telebot.ModeHTML})
	}

	msg := fmt.Sprintf(
		"<b>PR #%d</b> — %s\n\n"+
			"  Author: %s\n"+
			"  State: %s\n"+
			"  Platform: %s\n"+
			"  Repo Group: %s\n"+
			"  Labels: %s\n"+
			"  Spam: %v\n"+
			"  Created: %s\n",
		found.PRNumber, html.EscapeString(found.Title),
		html.EscapeString(found.Author), found.State, found.Platform,
		found.RepoGroup, html.EscapeString(strings.Join(found.Labels, ", ")),
		found.SpamFlag,
		found.CreatedAt.Format(time.RFC3339),
	)

	// Add action buttons
	selector := &telebot.ReplyMarkup{}
	btnApprove := selector.Data("✅ Approve", "approve", fmt.Sprintf("approve:%s#%s", repoGroup, found.ID))
	btnClose := selector.Data("❌ Close", "close", fmt.Sprintf("close:%s#%s", repoGroup, found.ID))
	btnSpam := selector.Data("🚫 Spam", "spam", fmt.Sprintf("spam:%s#%s", repoGroup, found.ID))
	btnReopen := selector.Data("🔄 Reopen", "reopen", fmt.Sprintf("reopen:%s#%s", repoGroup, found.ID))
	selector.Inline(selector.Row(btnApprove, btnClose), selector.Row(btnSpam, btnReopen))

	return c.Send(msg, &telebot.SendOptions{
		ParseMode:   telebot.ModeHTML,
		ReplyMarkup: selector,
	})
}

// handleApprovePR handles /approve command.
func (b *TelegramBot) handleApprovePR(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	if len(args) < 3 {
		return c.Send("Usage: /approve repo_group pr_id")
	}

	repoGroup := args[1]
	prID := args[2]

	pr, err := getPRByID(repoGroup, prID)
	if err != nil || pr == nil {
		return c.Send("PR not found.")
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return c.Send("Repo group not found.")
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		return c.Send(fmt.Sprintf("No client configured for platform %s.", pr.Platform))
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		return c.Send("Cannot resolve repository.")
	}

	ctx := context.Background()
	if err := client.ApprovePR(ctx, owner, repo, pr.PRNumber); err != nil {
		slog.Error("telegram bot: approve failed", "error", err)
		return c.Send(fmt.Sprintf("Failed to approve PR: %v", err))
	}

	return c.Send(fmt.Sprintf("PR #%d approved.", pr.PRNumber))
}

// handleClosePR handles /close command.
func (b *TelegramBot) handleClosePR(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	if len(args) < 3 {
		return c.Send("Usage: /close repo_group pr_id")
	}

	repoGroup := args[1]
	prID := args[2]

	pr, _ := getPRByID(repoGroup, prID)
	if pr == nil {
		return c.Send("PR not found.")
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return c.Send("Repo group not found.")
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		return c.Send("No client configured for platform.")
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)

	ctx := context.Background()
	if err := client.ClosePR(ctx, owner, repo, pr.PRNumber); err != nil {
		return c.Send(fmt.Sprintf("Failed to close PR: %v", err))
	}

	return c.Send(fmt.Sprintf("PR #%d closed.", pr.PRNumber))
}

// handleReopenPR handles /reopen command.
func (b *TelegramBot) handleReopenPR(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	if len(args) < 3 {
		return c.Send("Usage: /reopen repo_group pr_id")
	}

	repoGroup := args[1]
	prID := args[2]

	pr, _ := getPRByID(repoGroup, prID)
	if pr == nil {
		return c.Send("PR not found.")
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return c.Send("Repo group not found.")
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		return c.Send("No client configured for platform.")
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)

	ctx := context.Background()
	if err := client.ReopenPR(ctx, owner, repo, pr.PRNumber); err != nil {
		return c.Send(fmt.Sprintf("Failed to reopen PR: %v", err))
	}

	// If this was a spam recovery, clear spam flag
	if pr.SpamFlag {
		pr.State = "open"
		pr.SpamFlag = false
		pr.UpdatedAt = time.Now()
		data, _ := json.Marshal(pr)
		db.PutPRWithIndex(fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber), data, pr.ID, pr.RepoGroup, pr.PRNumber)
	}

	return c.Send(fmt.Sprintf("PR #%d reopened.", pr.PRNumber))
}

// handleMarkSpam handles /spam command.
func (b *TelegramBot) handleMarkSpam(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	if len(args) < 3 {
		return c.Send("Usage: /spam repo_group pr_id")
	}

	repoGroup := args[1]
	prID := args[2]

	pr, _ := getPRByID(repoGroup, prID)
	if pr == nil {
		return c.Send("PR not found.")
	}

	// Mark as spam
	pr.SpamFlag = true
	pr.State = "spam"
	pr.UpdatedAt = time.Now()

	key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
	data, _ := json.Marshal(pr)
	db.PutPRWithIndex(key, data, pr.ID, pr.RepoGroup, pr.PRNumber)

	// Close the PR on the platform
	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group != nil {
		client := b.getClientForPlatform(pr.Platform)
		if client != nil {
			owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
			client.ClosePR(context.Background(), owner, repo, pr.PRNumber)
		}
	}

	// Send notification
	if b.notifier != nil {
		title := fmt.Sprintf("[Spam Alert] PR #%d by %s", pr.PRNumber, pr.Author)
		body := fmt.Sprintf("PR #%d \"%s\" by %s marked as spam via Telegram.\nRepo: %s | Platform: %s",
			pr.PRNumber, pr.Title, pr.Author, pr.RepoGroup, pr.Platform)
		b.notifier.Send(context.Background(), title, body)
	}

	return c.Send(fmt.Sprintf("PR #%d marked as spam.", pr.PRNumber))
}

// handleShowQueue handles /queue command.
func (b *TelegramBot) handleShowQueue(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	repoGroup := ""
	if len(args) > 1 {
		repoGroup = args[1]
	} else {
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
		if repoGroup == "" || item.RepoGroup == repoGroup || strings.HasPrefix(string(key), repoGroup+"#") {
			items = append(items, item)
		}
		return nil
	})

	if len(items) == 0 {
		return c.Send(fmt.Sprintf("Queue empty for <b>%s</b>.", html.EscapeString(repoGroup)),
			&telebot.SendOptions{ParseMode: telebot.ModeHTML})
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Merge Queue — %s</b>\n\n", html.EscapeString(repoGroup)))
	for _, item := range items {
		statusEmoji := "⏳"
		switch item.Status {
		case "done":
			statusEmoji = "✅"
		case "failed":
			statusEmoji = "❌"
		case "merging":
			statusEmoji = "🔄"
		}
		sb.WriteString(fmt.Sprintf("%s %s (%s) — %s\n", statusEmoji, item.PRID, item.Status, item.AddedAt.Format(time.RFC3339)))
	}

	return c.Send(sb.String(), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

// handleRecheckQueue handles /recheck command.
func (b *TelegramBot) handleRecheckQueue(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	if b.queueMgr == nil {
		return c.Send("Queue manager not initialized.")
	}

	go b.queueMgr.CheckQueue()
	return c.Send("Queue recheck triggered.")
}

// handleShowConfig handles /config command.
func (b *TelegramBot) handleShowConfig(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	cfg := config.Current()
	if cfg == nil {
		return c.Send("Config not loaded.")
	}

	groups := config.GetRepoGroups(cfg)
	var sb strings.Builder
	sb.WriteString("<b>Current Config</b>\n\n")
	sb.WriteString(fmt.Sprintf("  Server: %s (%s)\n", cfg.Server.Listen, cfg.Server.Mode))
	sb.WriteString(fmt.Sprintf("  DB: %s\n", cfg.Database.Path))
	sb.WriteString(fmt.Sprintf("  Events: %s\n", cfg.Events.Mode))
	sb.WriteString(fmt.Sprintf("  Spam: enabled=%v\n", cfg.Spam.Enabled))
	sb.WriteString(fmt.Sprintf("  Notify channels: %d\n", len(cfg.Notify)))
	sb.WriteString(fmt.Sprintf("  Label rules: %d\n", len(cfg.LabelRules)))
	sb.WriteString(fmt.Sprintf("  Repo groups: %d\n", len(groups)))
	for _, g := range groups {
		sb.WriteString(fmt.Sprintf("    - %s (%s)\n", g.Name, g.Mode))
	}

	return c.Send(sb.String(), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

// handleCallback handles inline button callbacks.
func (b *TelegramBot) handleCallback(c telebot.Context) error {
	cb := c.Callback()
	if cb == nil {
		return nil
	}

	if !b.isAdmin(c) {
		return c.Respond(&telebot.CallbackResponse{Text: "Access denied."})
	}

	// telebot v3 sends callback data as \f<unique>|<data>
	// e.g., \freopen|reopen:default#241
	data := cb.Data

	// Strip the \f<unique>| prefix if present
	if len(data) > 0 && data[0] == '\f' {
		if idx := strings.Index(data, "|"); idx >= 0 {
			data = data[idx+1:]
		}
	}

	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid callback."})
	}

	action := parts[0]
	payload := parts[1]

	// payload format: repoGroup#prID
	idx := strings.LastIndex(payload, "#")
	if idx < 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid payload."})
	}
	repoGroup := payload[:idx]
	prID := payload[idx+1:]

	pr, err := getPRByID(repoGroup, prID)
	if err != nil || pr == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "PR not found."})
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Repo group not found."})
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "No platform client."})
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	ctx := context.Background()

	switch action {
	case "approve":
		if pr.State == "merged" || pr.State == "closed" {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("Cannot approve %s PR.", pr.State)})
		}
		if err := client.ApprovePR(ctx, owner, repo, pr.PRNumber); err != nil {
			msg := fmt.Sprintf("Failed: %v", err)
			if len(msg) > 200 {
				msg = msg[:197] + "..."
			}
			return c.Respond(&telebot.CallbackResponse{Text: msg})
		}
		pr.IsApproved = true
		prData, _ := json.Marshal(pr)
		key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
		db.PutPRWithIndex(key, prData, pr.ID, pr.RepoGroup, pr.PRNumber)
		c.Respond(&telebot.CallbackResponse{Text: "Approved ✅"})

	case "close":
		if pr.State == "closed" || pr.State == "merged" {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("PR is already %s.", pr.State)})
		}
		if err := client.ClosePR(ctx, owner, repo, pr.PRNumber); err != nil {
			msg := fmt.Sprintf("Failed: %v", err)
			if len(msg) > 200 {
				msg = msg[:197] + "..."
			}
			return c.Respond(&telebot.CallbackResponse{Text: msg})
		}
		pr.State = "closed"
		prData, _ := json.Marshal(pr)
		key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
		db.PutPRWithIndex(key, prData, pr.ID, pr.RepoGroup, pr.PRNumber)
		c.Respond(&telebot.CallbackResponse{Text: "Closed ❌"})

	case "reopen":
		if pr.State == "merged" {
			return c.Respond(&telebot.CallbackResponse{Text: "Cannot reopen merged PR."})
		}
		if pr.State == "open" {
			return c.Respond(&telebot.CallbackResponse{Text: "PR is already open."})
		}
		if err := client.ReopenPR(ctx, owner, repo, pr.PRNumber); err != nil {
			msg := fmt.Sprintf("Failed: %v", err)
			if len(msg) > 200 {
				msg = msg[:197] + "..."
			}
			return c.Respond(&telebot.CallbackResponse{Text: msg})
		}
		if pr.SpamFlag {
			pr.State = "open"
			pr.SpamFlag = false
		} else {
			pr.State = "open"
		}
		pr.UpdatedAt = time.Now()
		data, _ := json.Marshal(pr)
		key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
		db.PutPRWithIndex(key, data, pr.ID, pr.RepoGroup, pr.PRNumber)
		c.Respond(&telebot.CallbackResponse{Text: "Reopened 🔄"})

	case "spam":
		if pr.State == "closed" || pr.State == "merged" {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("PR is already %s.", pr.State)})
		}
		pr.SpamFlag = true
		pr.State = "spam"
		pr.UpdatedAt = time.Now()
		data, _ := json.Marshal(pr)
		key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
		db.PutPRWithIndex(key, data, pr.ID, pr.RepoGroup, pr.PRNumber)
		if err := client.ClosePR(ctx, owner, repo, pr.PRNumber); err != nil {
			msg := fmt.Sprintf("Marked spam but close failed: %v", err)
			if len(msg) > 200 {
				msg = msg[:197] + "..."
			}
			return c.Respond(&telebot.CallbackResponse{Text: msg})
		}
		c.Respond(&telebot.CallbackResponse{Text: "Marked as spam 🚫"})

	default:
		return c.Respond(&telebot.CallbackResponse{Text: "Unknown action."})
	}

	return nil
}

// handleText handles non-command text messages.
func (b *TelegramBot) handleText(c telebot.Context) error {
	text := strings.TrimSpace(c.Text())
	lower := strings.ToLower(text)

	switch lower {
	case "help", "menu":
		return b.handleHelp(c)
	case "prs", "list":
		return b.handleListPRs(c)
	case "queue":
		return b.handleShowQueue(c)
	case "config":
		return b.handleShowConfig(c)
	}

	c.Send("Unknown command. Try /help for available commands.")
	return nil
}

// getClientForPlatform returns the platform client.
func (b *TelegramBot) getClientForPlatform(platform string) platforms.PlatformClient {
	if b.clients == nil {
		return nil
	}
	return b.clients[platforms.PlatformType(platform)]
}

// getPRByID finds a PR by repo group and ID/number.
func getPRByID(repoGroup, idOrNumber string) (*models.PRRecord, error) {
	var found *models.PRRecord
	prNumber, _ := strconv.Atoi(idOrNumber)

	db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if err := json.Unmarshal(value, &pr); err != nil {
			return nil
		}
		if pr.RepoGroup == repoGroup {
			if pr.ID == idOrNumber || prNumber > 0 && pr.PRNumber == prNumber {
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

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (b *TelegramBot) handleStaleCheck(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	repoGroup := ""
	if len(args) > 1 {
		repoGroup = args[1]
	}

	dryRun := false
	if len(args) > 2 && args[2] == "--dry-run" {
		dryRun = true
	}

	cfg := config.Current()
	if cfg == nil || !cfg.Stale.Enabled {
		return c.Send("Stale PR management is not enabled in config.")
	}

	var groups []models.RepoGroup
	if repoGroup != "" {
		g := config.GetRepoGroupByName(cfg, repoGroup)
		if g == nil {
			return c.Send("Repo group not found: " + repoGroup)
		}
		groups = []models.RepoGroup{*g}
	} else {
		groups = config.GetRepoGroups(cfg)
	}

	var lines []string
	if dryRun {
		lines = append(lines, "<b>Stale PR Dry Run:</b>")
	} else {
		lines = append(lines, "<b>Stale PR Check Results:</b>")
	}

	for _, group := range groups {
		prs, err := b.fetchOpenPRs(&group)
		if err != nil {
			lines = append(lines, fmt.Sprintf("- %s: error listing PRs", group.Name))
			continue
		}
		for _, pr := range prs {
			days := inactivityDays(pr.UpdatedAt)
			hasStale := hasLabelStr(pr.Labels, cfg.Stale.StaleLabel, "stale")
			isExempt := false
			for _, exempt := range cfg.Stale.ExemptLabels {
				if hasLabelStr(pr.Labels, exempt, "") {
					isExempt = true
					break
				}
			}
			if cfg.Stale.SkipDraftPRs && pr.IsDraft {
				continue
			}
			if isExempt {
				continue
			}
			if hasStale && cfg.Stale.DaysUntilClose > 0 && days >= cfg.Stale.DaysUntilStale+cfg.Stale.DaysUntilClose {
				lines = append(lines, fmt.Sprintf("- [CLOSE] #%d %s (%s, %dd stale)",
					pr.PRNumber, html.EscapeString(truncate(pr.Title, 40)), group.Name, days))
			} else if !hasStale && days >= cfg.Stale.DaysUntilStale {
				lines = append(lines, fmt.Sprintf("- [MARK] #%d %s (%s, %dd inactive)",
					pr.PRNumber, html.EscapeString(truncate(pr.Title, 40)), group.Name, days))
			}
		}
	}

	if len(lines) == 1 {
		return c.Send("No stale PRs found.")
	}
	return c.Send(strings.Join(lines, "\n"), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

func (b *TelegramBot) handleUnstale(c telebot.Context) error {
	if !b.requireAdmin(c) {
		return nil
	}

	args := strings.Fields(c.Text())
	if len(args) < 3 {
		return c.Send("Usage: /unstale repo_group pr_number")
	}

	repoGroup := args[1]
	prNumber := args[2]

	cfg := config.Current()
	if cfg == nil {
		return c.Send("Config not loaded.")
	}

	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		return c.Send("Repo group not found: " + repoGroup)
	}

	label := cfg.Stale.StaleLabel
	if label == "" {
		label = "stale"
	}

	removed := false
	for _, pt := range groupPlatforms(group) {
		client, ok := b.clients[pt]
		if !ok {
			continue
		}
		owner, repo := config.GetOwnerRepoFromGroup(group, string(pt))
		if owner == "" || repo == "" {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := client.RemoveLabel(ctx, owner, repo, parseInt(prNumber), label)
		cancel()
		if err != nil {
			continue
		}
		removed = true
	}

	if !removed {
		return c.Send("Failed to remove stale label.")
	}
	return c.Send("Stale label removed from PR #" + prNumber + " in " + repoGroup)
}

func (b *TelegramBot) fetchOpenPRs(group *models.RepoGroup) ([]*models.PRRecord, error) {
	var prs []*models.PRRecord
	for _, pt := range groupPlatforms(group) {
		client, ok := b.clients[pt]
		if !ok {
			continue
		}
		owner, repo := config.GetOwnerRepoFromGroup(group, string(pt))
		if owner == "" || repo == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		list, err := client.ListPRs(ctx, owner, repo, "open")
		cancel()
		if err != nil {
			return nil, err
		}
		prs = append(prs, list...)
	}
	return prs, nil
}

func groupPlatforms(group *models.RepoGroup) []platforms.PlatformType {
	var result []platforms.PlatformType
	if group.GitHub != "" {
		result = append(result, platforms.PlatformGitHub)
	}
	if group.GitLab != "" {
		result = append(result, platforms.PlatformGitLab)
	}
	if group.Gitea != "" {
		result = append(result, platforms.PlatformGitea)
	}
	return result
}

func inactivityDays(lastActive time.Time) int {
	dur := time.Since(lastActive)
	return int(dur.Hours() / 24)
}

func hasLabelStr(labels []string, target, defaultLabel string) bool {
	check := target
	if check == "" {
		check = defaultLabel
	}
	for _, l := range labels {
		if l == check {
			return true
		}
	}
	return false
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
