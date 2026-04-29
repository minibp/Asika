package labeler

import (
	"context"
	"log/slog"
	"path"
	"strings"

	"asika/common/config"
	"asika/common/events"
	"asika/common/models"
	"asika/common/platforms"
)

// Labeler handles label rule application
type Labeler struct {
	clients map[platforms.PlatformType]platforms.PlatformClient
}

// NewLabeler creates a new labeler
func NewLabeler(clients map[platforms.PlatformType]platforms.PlatformClient) *Labeler {
	return &Labeler{
		clients: clients,
	}
}

// HandlePROpened handles PR opened event
func (l *Labeler) HandlePROpened(pr *models.PRRecord, repoGroup string) {
	cfg := config.Current()
	if cfg == nil {
		return
	}
	rules := cfg.LabelRules

	if len(rules) == 0 {
		return
	}

	// TODO: get PR changed files from platform
	// For now, just log
	slog.Info("applying label rules", "pr", pr.Title, "rules", len(rules))

	// TODO: implement actual label application
	// 1. Get PR changed files
	// 2. Match against rules
	// 3. Call platform.AddLabel() for matched labels
}

// ApplyRules applies label rules to a PR
func (l *Labeler) ApplyRules(pr *models.PRRecord, repoGroup string, files []string) {
	cfg := config.Current()
	if cfg == nil {
		return
	}
	rules := cfg.LabelRules

	client, ok := l.clients[platforms.PlatformType(pr.Platform)]
	if !ok {
		slog.Error("no client for platform", "platform", pr.Platform)
		return
	}

	// Get repo info from config
	repoGroupConfig := getRepoGroupConfig(cfg, repoGroup)
	if repoGroupConfig == nil {
		return
	}

	owner, repo := getOwnerAndRepo(repoGroupConfig, pr.Platform)
	if owner == "" || repo == "" {
		return
	}

	for _, rule := range rules {
		if matchPattern(rule.Pattern, files) {
			slog.Info("adding label", "label", rule.Label, "pr", pr.PRNumber)
			ctx := context.Background()
			if err := client.AddLabel(ctx, owner, repo, pr.PRNumber, rule.Label); err != nil {
				slog.Error("failed to add label", "error", err, "label", rule.Label)
			}
		}
	}
}

func getRepoGroupConfig(cfg *models.Config, name string) *models.RepoGroupConfig {
	for _, rg := range cfg.RepoGroups {
		if rg.Name == name {
			return &rg
		}
	}
	return nil
}

func getOwnerAndRepo(rg *models.RepoGroupConfig, platform string) (string, string) {
	var repoPath string
	switch platform {
	case "github":
		repoPath = rg.GitHub
	case "gitlab":
		repoPath = rg.GitLab
	case "gitea":
		repoPath = rg.Gitea
	}

	// Parse owner/repo
	idx := strings.LastIndex(repoPath, "/")
	if idx < 0 {
		return "", repoPath
	}
	return repoPath[:idx], repoPath[idx+1:]
}

func matchPattern(pattern string, files []string) bool {
	for _, file := range files {
		if matchSinglePattern(pattern, file) {
			return true
		}
	}
	return false
}

func matchSinglePattern(pattern, file string) bool {
	// Simple glob matching
	// TODO: use github.com/gobwas/glob or path.Match
	matched, _ := path.Match(pattern, file)
	return matched
}

// Subscribe subscribes to events
func (l *Labeler) Subscribe() {
	ch := events.Subscribe()
	go func() {
		for event := range ch {
			if event.Type == events.EventPROpened && event.PR != nil {
				l.HandlePROpened(event.PR, event.RepoGroup)
			}
		}
	}()
}
