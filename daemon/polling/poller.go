package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"asika/common/db"
	"asika/common/events"
	"asika/common/models"
	"asika/common/platforms"
)

// Poller polls platforms for PR changes
type Poller struct {
	cfg     *models.Config
	clients map[platforms.PlatformType]platforms.PlatformClient
	stop    chan struct{}
}

// NewPoller creates a new poller
func NewPoller(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) *Poller {
	return &Poller{
		cfg:     cfg,
		clients: clients,
		stop:    make(chan struct{}),
	}
}

// Start starts the polling loop
func (p *Poller) Start() {
	interval := parseDuration(p.cfg.Events.PollingInterval, 30*time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("polling started", "interval", interval)

	for {
		select {
		case <-ticker.C:
			p.pollOnce()
		case <-p.stop:
			slog.Info("polling stopped")
			return
		}
	}
}

// PollOnce performs a single poll cycle (can be called externally for initial fetch)
func (p *Poller) PollOnce() {
	p.pollOnce()
}

// Stop stops the poller
func (p *Poller) Stop() {
	close(p.stop)
}

func (p *Poller) pollOnce() {
	var success, failed int
	for _, repoGroup := range p.cfg.RepoGroups {
		s, f := p.pollRepoGroup(repoGroup)
		success += s
		failed += f
	}
	if total := success + failed; total > 0 {
		slog.Info("PR fetch complete", "total", total, "success", success, "failed", failed)
	}
}

func (p *Poller) pollRepoGroup(rg models.RepoGroupConfig) (success, failed int) {
	platforms := []struct {
		ptype platforms.PlatformType
		repo  string
	}{
		{platforms.PlatformGitHub, rg.GitHub},
		{platforms.PlatformGitLab, rg.GitLab},
		{platforms.PlatformGitea, rg.Gitea},
	}

	for _, pinfo := range platforms {
		if pinfo.repo == "" {
			continue
		}
		client, ok := p.clients[pinfo.ptype]
		if !ok {
			continue
		}

		s, f := p.pollPlatform(client, rg.Name, string(pinfo.ptype), pinfo.repo)
		success += s
		failed += f
	}
	return
}

func (p *Poller) pollPlatform(client platforms.PlatformClient, repoGroup, platform, repo string) (success, failed int) {
	ctx := context.Background()

	// Parse owner/repo using the same logic as config.GetOwnerRepoFromGroup
	idx := strings.LastIndex(repo, "/")
	owner := ""
	repoName := repo
	if idx >= 0 {
		owner = repo[:idx]
		repoName = repo[idx+1:]
	}

	prs, err := client.ListPRs(ctx, owner, repoName, "all")
	if err != nil {
		slog.Error("failed to list PRs", "platform", platform, "repo", repo, "error", err)
		return 0, 1
	}

	// Compare with local DB and publish events
	for _, pr := range prs {
		pr.RepoGroup = repoGroup
		pr.Platform = platform

		// GitHub API returns merged PRs as "closed"; detect via MergedAt
		if pr.State == "closed" && !pr.MergedAt.IsZero() {
			pr.State = "merged"
		}

		// Check if PR exists in DB
		key := fmt.Sprintf("%s#%s#%d", repoGroup, platform, pr.PRNumber)
		data, _ := db.Get(db.BucketPRs, key)

		if data == nil {
			pr.CreatedAt = time.Now()
			pr.UpdatedAt = time.Now()
			events.PublishPR(events.EventPROpened, repoGroup, platform, pr, nil)
		} else {
			// Check if updated
			var existing models.PRRecord
			if err := json.Unmarshal(data, &existing); err == nil {
				if existing.State != pr.State {
					switch pr.State {
					case "open":
						events.PublishPR(events.EventPROpened, repoGroup, platform, pr, nil)
					case "closed":
						events.PublishPR(events.EventPRClosed, repoGroup, platform, pr, nil)
					case "merged":
						events.PublishPR(events.EventPRMerged, repoGroup, platform, pr, nil)
					}
				}
			}
		}

		// Store/update in DB
		if pr.ID == "" {
			pr.ID = uuid.New().String()
		}
		prData, _ := json.Marshal(pr)
		if err := db.PutPRWithIndex(key, prData, pr.ID, pr.RepoGroup, pr.PRNumber); err != nil {
			slog.Error("failed to save PR", "pr", pr.PRNumber, "platform", platform, "error", err)
			failed++
		} else {
			success++
		}
	}
	return
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}
