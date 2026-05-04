package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/platforms"
)

// Checker checks if a queue item is ready to merge
type Checker struct {
	cfg     *models.Config
	clients map[platforms.PlatformType]platforms.PlatformClient
}

// NewChecker creates a new checker
func NewChecker(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) *Checker {
	return &Checker{
		cfg:     cfg,
		clients: clients,
	}
}

// ShouldMerge checks if a queue item should be merged
func (c *Checker) ShouldMerge(item *models.QueueItem) (bool, error) {
	ctx := context.Background()

	pr, err := getPRFromDB(item.RepoGroup, item.PRID)
	if err != nil {
		return false, err
	}

	// Get repo group config
	group := config.GetRepoGroupByName(c.cfg, pr.RepoGroup)
	if group == nil {
		return false, fmt.Errorf("repo group not found: %s", pr.RepoGroup)
	}

	// Check for merge conflicts
	if pr.HasConflict {
		slog.Info("PR has merge conflicts, skipping", "pr_id", pr.ID, "title", pr.Title)
		return false, nil
	}

	// Check approvals
	approved, approvals, err := c.checkApprovals(ctx, pr, group)
	if err != nil {
		return false, err
	}
	if !approved {
		return false, nil
	}

	// Check CI (skip if ci_check_required is false or ci_provider is "none")
	if group.MergeQueue.CICheckRequired && group.CIProvider != "none" {
		ciPassed, ciStatus, err := c.checkCI(ctx, pr, group)
		if err != nil {
			return false, err
		}
		if !ciPassed {
			// Update criteria snapshot
			item.Criteria = models.MergeCriteria{
				RequiredApprovals: group.MergeQueue.RequiredApprovals,
				ApprovedBy:        approvals,
				CIStatus:          ciStatus,
			}
			return false, nil
		}
	}

	// Update criteria snapshot
	item.Criteria = models.MergeCriteria{
		RequiredApprovals: group.MergeQueue.RequiredApprovals,
		ApprovedBy:        approvals,
		CIStatus:          "success",
	}

	return true, nil
}

// checkApprovals checks if the PR has enough approvals from core contributors
func (c *Checker) checkApprovals(ctx context.Context, pr *models.PRRecord, group *models.RepoGroup) (bool, []string, error) {
	client := c.clients[platforms.PlatformType(pr.Platform)]
	if client == nil {
		return false, nil, fmt.Errorf("no client for platform: %s", pr.Platform)
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		return false, nil, fmt.Errorf("cannot resolve repo for platform %s in group %s", pr.Platform, group.Name)
	}

	approvals, err := client.GetApprovals(ctx, owner, repo, pr.PRNumber)
	if err != nil {
		return false, nil, err
	}

	// Check if core contributors approved
	coreApproved := make([]string, 0)
	for _, approver := range approvals {
		if contains(group.MergeQueue.CoreContributors, approver) {
			coreApproved = append(coreApproved, approver)
		}
	}

	return len(coreApproved) >= group.MergeQueue.RequiredApprovals, coreApproved, nil
}

// checkCI checks if CI passed
func (c *Checker) checkCI(ctx context.Context, pr *models.PRRecord, group *models.RepoGroup) (bool, string, error) {
	client := c.clients[platforms.PlatformType(pr.Platform)]
	if client == nil {
		return false, "none", fmt.Errorf("no client for platform: %s", pr.Platform)
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		return false, "none", fmt.Errorf("cannot resolve repo for platform %s", pr.Platform)
	}

	// Get the latest commit SHA from the PR
	commits, err := client.GetPRCommits(ctx, owner, repo, pr.PRNumber)
	if err != nil {
		return false, "none", err
	}
	if len(commits) == 0 {
		return true, "none", nil
	}

	lastCommit := commits[len(commits)-1]
	status, err := client.GetCIStatus(ctx, owner, repo, lastCommit)
	if err != nil {
		return false, "none", err
	}

	return status == "success", status, nil
}

func getPRFromDB(repoGroup, prID string) (*models.PRRecord, error) {
	// Try to find PR by iterating through the bucket
	var pr *models.PRRecord
	err := db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var record models.PRRecord
		if err := json.Unmarshal(value, &record); err != nil {
			return err
		}
		if record.RepoGroup == repoGroup && record.ID == prID {
			pr = &record
			return nil
		}
		// Also try matching by PR number as key format is repoGroup#platform#prNumber
		return nil
	})
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return nil, fmt.Errorf("PR not found: %s/%s", repoGroup, prID)
	}
	return pr, nil
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}