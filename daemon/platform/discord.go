package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
	"asika/daemon/queue"
	"asika/daemon/syncer"
)

// DiscordBot wraps the Discord bot with Asika management functionality
type DiscordBot struct {
	session      *discordgo.Session
	cfg          *models.Config
	clients      map[platforms.PlatformType]platforms.PlatformClient
	queueMgr     *queue.Manager
	syncerRef    *syncer.Syncer
	spamDetector *syncer.SpamDetector
	notifier     *notifier.DiscordNotifier
	adminIDs     map[string]bool
	stop         chan struct{}
}

// SetSession sets the Discord session
func (b *DiscordBot) SetSession(s *discordgo.Session) {
	b.session = s
}

// NewDiscordBot creates a new Discord bot
func NewDiscordBot(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncerRef *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
	discordNotifier *notifier.DiscordNotifier,
	adminIDs []string,
) *DiscordBot {
	b := &DiscordBot{
		cfg:          cfg,
		clients:      clients,
		queueMgr:     queueMgr,
		syncerRef:    syncerRef,
		spamDetector: spamDetector,
		notifier:     discordNotifier,
		adminIDs:     make(map[string]bool),
		stop:         make(chan struct{}),
	}
	for _, id := range adminIDs {
		b.adminIDs[id] = true
	}
	return b
}

// Start starts the bot and registers command handlers
func (b *DiscordBot) Start() {
	if b.session == nil {
		slog.Warn("discord bot: no session, skipping start")
		return
	}

	slog.Info("starting discord interactive bot")

	b.registerCommands()

	go func() {
		b.session.Open()
	}()
}

// Stop stops the bot gracefully
func (b *DiscordBot) Stop() {
	close(b.stop)
	if b.session != nil {
		b.session.Close()
	}
	slog.Info("discord bot stopped")
}

// registerCommands registers all bot command handlers
func (b *DiscordBot) registerCommands() {
	b.session.AddHandler(b.handleMessageCreate)
}

// isAdmin checks if the sender is an authorized admin
func (b *DiscordBot) isAdmin(userID string) bool {
	if len(b.adminIDs) == 0 {
		return true
	}
	return b.adminIDs[userID]
}

func (b *DiscordBot) requireAdmin(userID string) bool {
	if !b.isAdmin(userID) {
		return false
	}
	return true
}

// handleMessageCreate handles incoming messages
func (b *DiscordBot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if !b.requireAdmin(m.Author.ID) {
		return
	}

	content := strings.TrimSpace(m.Content)
	if content == "" {
		return
	}

	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "!help":
		b.handleHelp(s, m)
	case "!prs":
		b.handleListPRs(s, m, parts)
	case "!pr":
		b.handleShowPR(s, m, parts)
	case "!approve":
		b.handleApprovePR(s, m, parts)
	case "!close":
		b.handleClosePR(s, m, parts)
	case "!reopen":
		b.handleReopenPR(s, m, parts)
	case "!spam":
		b.handleMarkSpam(s, m, parts)
	case "!queue":
		b.handleShowQueue(s, m, parts)
	case "!recheck":
		b.handleRecheckQueue(s, m)
	case "!config":
		b.handleShowConfig(s, m)
	default:
		if strings.HasPrefix(cmd, "!") {
			s.ChannelMessageSend(m.ChannelID, "Unknown command. Use !help for available commands.")
		}
	}
}

// handleHelp handles !help command
func (b *DiscordBot) handleHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	help := `**Asika Bot Commands**

**PR Management**
!prs [repo_group] — List PRs
!pr <repo_group> <number> — Show PR details
!approve <repo_group> <pr_id> — Approve a PR
!close <repo_group> <pr_id> — Close a PR
!reopen <repo_group> <pr_id> — Reopen a PR (spam recovery)
!spam <repo_group> <pr_id> — Mark PR as spam

**Queue**
!queue [repo_group] — Show merge queue
!recheck [repo_group] — Trigger queue recheck

**Config**
!config — Show current config (masked)`
	s.ChannelMessageSend(m.ChannelID, help)
}

// handleListPRs handles !prs command
func (b *DiscordBot) handleListPRs(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	repoGroup := ""
	if len(args) > 1 {
		repoGroup = args[1]
	} else {
		groups := config.GetRepoGroups(b.cfg)
		if len(groups) == 0 {
			s.ChannelMessageSend(m.ChannelID, "No repo groups configured.")
			return
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
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("No PRs found for repo group **%s**.", repoGroup))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**PRs in %s**\n\n", repoGroup))
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
		sb.WriteString(fmt.Sprintf("%s **#%d** %s — by %s (%s/%s)\n",
			statusEmoji, pr.PRNumber, truncate(pr.Title, 40), pr.Author, pr.Platform, pr.State))
	}

	s.ChannelMessageSend(m.ChannelID, sb.String())
}

// handleShowPR handles !pr command
func (b *DiscordBot) handleShowPR(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!pr <repo_group> <pr_number>`")
		return
	}

	repoGroup := args[1]
	prNumber, err := strconv.Atoi(args[2])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid PR number.")
		return
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
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("PR #%d not found in repo group **%s**.", prNumber, repoGroup))
		return
	}

	msg := fmt.Sprintf(
		"**PR #%d** — %s\n\n"+
			"  Author: %s\n"+
			"  State: %s\n"+
			"  Platform: %s\n"+
			"  Repo Group: %s\n"+
			"  Labels: %s\n"+
			"  Spam: %v\n"+
			"  Created: %s\n",
		found.PRNumber, found.Title,
		found.Author, found.State, found.Platform,
		found.RepoGroup, strings.Join(found.Labels, ", "),
		found.SpamFlag,
		found.CreatedAt.Format(time.RFC3339),
	)

	s.ChannelMessageSend(m.ChannelID, msg)
}

// handleApprovePR handles !approve command
func (b *DiscordBot) handleApprovePR(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!approve <repo_group> <pr_id>`")
		return
	}

	repoGroup := args[1]
	prID := args[2]

	pr, err := getPRByID(repoGroup, prID)
	if err != nil || pr == nil {
		s.ChannelMessageSend(m.ChannelID, "PR not found.")
		return
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		s.ChannelMessageSend(m.ChannelID, "Repo group not found.")
		return
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("No client configured for platform %s.", pr.Platform))
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)

	ctx := context.Background()
	if err := client.ApprovePR(ctx, owner, repo, pr.PRNumber); err != nil {
		slog.Error("discord bot: approve failed", "error", err)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to approve PR: %v", err))
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("PR #%d approved.", pr.PRNumber))
}

// handleClosePR handles !close command
func (b *DiscordBot) handleClosePR(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!close <repo_group> <pr_id>`")
		return
	}

	repoGroup := args[1]
	prID := args[2]

	pr, _ := getPRByID(repoGroup, prID)
	if pr == nil {
		s.ChannelMessageSend(m.ChannelID, "PR not found.")
		return
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		s.ChannelMessageSend(m.ChannelID, "Repo group not found.")
		return
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		s.ChannelMessageSend(m.ChannelID, "No client configured for platform.")
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)

	ctx := context.Background()
	if err := client.ClosePR(ctx, owner, repo, pr.PRNumber); err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to close PR: %v", err))
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("PR #%d closed.", pr.PRNumber))
}

// handleReopenPR handles !reopen command
func (b *DiscordBot) handleReopenPR(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!reopen <repo_group> <pr_id>`")
		return
	}

	repoGroup := args[1]
	prID := args[2]

	pr, _ := getPRByID(repoGroup, prID)
	if pr == nil {
		s.ChannelMessageSend(m.ChannelID, "PR not found.")
		return
	}

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group == nil {
		s.ChannelMessageSend(m.ChannelID, "Repo group not found.")
		return
	}

	client := b.getClientForPlatform(pr.Platform)
	if client == nil {
		s.ChannelMessageSend(m.ChannelID, "No client configured for platform.")
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)

	ctx := context.Background()
	if err := client.ReopenPR(ctx, owner, repo, pr.PRNumber); err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to reopen PR: %v", err))
		return
	}

	if pr.SpamFlag {
		pr.State = "open"
		pr.SpamFlag = false
		pr.UpdatedAt = time.Now()
		data, _ := json.Marshal(pr)
		db.PutPRWithIndex(fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber), data, pr.ID, pr.RepoGroup, pr.PRNumber)
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("PR #%d reopened.", pr.PRNumber))
}

// handleMarkSpam handles !spam command
func (b *DiscordBot) handleMarkSpam(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!spam <repo_group> <pr_id>`")
		return
	}

	repoGroup := args[1]
	prID := args[2]

	pr, _ := getPRByID(repoGroup, prID)
	if pr == nil {
		s.ChannelMessageSend(m.ChannelID, "PR not found.")
		return
	}

	pr.SpamFlag = true
	pr.State = "spam"
	pr.UpdatedAt = time.Now()

	key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
	data, _ := json.Marshal(pr)
	db.PutPRWithIndex(key, data, pr.ID, pr.RepoGroup, pr.PRNumber)

	group := config.GetRepoGroupByName(b.cfg, repoGroup)
	if group != nil {
		client := b.getClientForPlatform(pr.Platform)
		if client != nil {
			owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
			client.ClosePR(context.Background(), owner, repo, pr.PRNumber)
		}
	}

	if b.notifier != nil {
		title := fmt.Sprintf("[Spam Alert] PR #%d by %s", pr.PRNumber, pr.Author)
		body := fmt.Sprintf("PR #%d \"%s\" by %s marked as spam via Discord.\nRepo: %s | Platform: %s",
			pr.PRNumber, pr.Title, pr.Author, pr.RepoGroup, pr.Platform)
		b.notifier.Send(context.Background(), title, body)
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("PR #%d marked as spam.", pr.PRNumber))
}

// handleShowQueue handles !queue command
func (b *DiscordBot) handleShowQueue(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
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
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Queue empty for **%s**.", repoGroup))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Merge Queue — %s**\n\n", repoGroup))
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

	s.ChannelMessageSend(m.ChannelID, sb.String())
}

// handleRecheckQueue handles !recheck command
func (b *DiscordBot) handleRecheckQueue(s *discordgo.Session, m *discordgo.MessageCreate) {
	if b.queueMgr == nil {
		s.ChannelMessageSend(m.ChannelID, "Queue manager not initialized.")
		return
	}

	go b.queueMgr.CheckQueue()
	s.ChannelMessageSend(m.ChannelID, "Queue recheck triggered.")
}

// handleShowConfig handles !config command
func (b *DiscordBot) handleShowConfig(s *discordgo.Session, m *discordgo.MessageCreate) {
	cfg := config.Current()
	if cfg == nil {
		s.ChannelMessageSend(m.ChannelID, "Config not loaded.")
		return
	}

	groups := config.GetRepoGroups(cfg)
	var sb strings.Builder
	sb.WriteString("**Current Config**\n\n")
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

	s.ChannelMessageSend(m.ChannelID, sb.String())
}

// getClientForPlatform returns the platform client
func (b *DiscordBot) getClientForPlatform(platform string) platforms.PlatformClient {
	if b.clients == nil {
		return nil
	}
	return b.clients[platforms.PlatformType(platform)]
}
