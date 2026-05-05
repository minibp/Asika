package labeler

import (
	"context"
	"log/slog"
	"path"
	"regexp"
	"strings"

	"asika/common/config"
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

// HandlePROpened handles PR opened event by fetching diff files and applying rules
func (l *Labeler) HandlePROpened(pr *models.PRRecord, repoGroup string) {
	cfg := config.Current()
	if cfg == nil {
		return
	}
	rules := cfg.LabelRules
	if len(rules) == 0 {
		return
	}

	client, ok := l.clients[platforms.PlatformType(pr.Platform)]
	if !ok {
		slog.Error("no client for platform", "platform", pr.Platform)
		return
	}

	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		slog.Error("repo group not found", "name", repoGroup)
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		return
	}

	ctx := context.Background()
	files, err := client.GetDiffFiles(ctx, owner, repo, pr.PRNumber)
	if err != nil {
		slog.Error("failed to get diff files", "error", err, "platform", pr.Platform)
		return
	}

	l.ApplyRules(pr, repoGroup, files)
}

// ApplyRules applies label rules to a PR based on its changed files, title, and description
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

	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		return
	}

	ctx := context.Background()
	for _, rule := range rules {
		if matchRule(rule.Pattern, files, pr.Title, pr.Author) {
			slog.Info("adding label", "label", rule.Label, "pr", pr.PRNumber, "pattern", rule.Pattern)
			color := rule.Color
			if color == "" {
				color = "ededed"
			}
			if err := client.AddLabel(ctx, owner, repo, pr.PRNumber, rule.Label, color); err != nil {
				slog.Error("failed to add label", "error", err, "label", rule.Label)
			}
		}
	}
}

func matchRule(pattern string, files []string, title, author string) bool {
	scope := "file"
	pat := pattern

	if idx := strings.Index(pattern, ":"); idx > 0 && idx < 10 {
		prefix := strings.ToLower(pattern[:idx])
		if prefix == "title" || prefix == "author" || prefix == "file" {
			scope = prefix
			pat = pattern[idx+1:]
		}
	}

	var targets []string
	switch scope {
	case "title":
		targets = []string{title}
	case "author":
		targets = []string{author}
	default:
		targets = files
	}

	for _, target := range targets {
		if matchSinglePattern(pat, target) {
			return true
		}
	}
	return false
}

func matchPattern(pattern string, files []string) bool {
	for _, file := range files {
		if matchSinglePattern(pattern, file) {
			return true
		}
	}
	return false
}

var compiledPatterns = make(map[string]*regexp.Regexp)

func matchSinglePattern(pattern, file string) bool {
	if strings.ContainsAny(pattern, "*?[") {
		matched, _ := path.Match(pattern, file)
		if matched {
			return true
		}
	}

	re, ok := compiledPatterns[pattern]
	if !ok {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return false
		}
		compiledPatterns[pattern] = re
	}
	return re.MatchString(file)
}
