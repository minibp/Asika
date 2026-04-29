package queue

import (
    "context"
    "encoding/json"
    "fmt"

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

    // Get PR
    pr, err := getPRFromDB(item.RepoGroup, item.PRID)
    if err != nil {
        return false, err
    }

    // Check approvals
    approved, err := c.checkApprovals(ctx, pr)
    if err != nil {
        return false, err
    }
    if !approved {
        return false, nil
    }

    // Check CI
    ciPassed, err := c.checkCI(ctx, pr)
    if err != nil {
        return false, err
    }
    if !ciPassed {
        return false, nil
    }

    return true, nil
}

// checkApprovals checks if the PR has enough approvals
func (c *Checker) checkApprovals(ctx context.Context, pr *models.PRRecord) (bool, error) {
    // Get repo group config for core contributors
    group := config.GetRepoGroupByName(c.cfg, pr.RepoGroup)
    if group == nil {
        return false, fmt.Errorf("repo group not found: %s", pr.RepoGroup)
    }

    // Get platform client
    client := c.clients[platforms.PlatformType(pr.Platform)]
    if client == nil {
        return false, fmt.Errorf("no client for platform: %s", pr.Platform)
    }

    // Get approvals from platform
    parts := splitRepoName(pr.RepoGroup)
    if len(parts) != 2 {
        return false, fmt.Errorf("invalid repo group: %s", pr.RepoGroup)
    }

    approvals, err := client.GetApprovals(ctx, parts[0], parts[1], pr.PRNumber)
    if err != nil {
        return false, err
    }

    // Check if core contributors approved
    approvedCount := 0
    for _, approver := range approvals {
        if contains(group.MergeQueue.CoreContributors, approver) {
            approvedCount++
        }
    }

    return approvedCount >= group.MergeQueue.RequiredApprovals, nil
}

// checkCI checks if CI passed
func (c *Checker) checkCI(ctx context.Context, pr *models.PRRecord) (bool, error) {
    group := config.GetRepoGroupByName(c.cfg, pr.RepoGroup)
    if group == nil {
        return false, fmt.Errorf("repo group not found: %s", pr.RepoGroup)
    }

    // If CI provider is none, skip check
    if group.CIProvider == "none" {
        return true, nil
    }

    // Get platform client
    client := c.clients[platforms.PlatformType(pr.Platform)]
    if client == nil {
        return false, fmt.Errorf("no client for platform: %s", pr.Platform)
    }

    // Get CI status
    parts := splitRepoName(pr.RepoGroup)
    if len(parts) != 2 {
        return false, fmt.Errorf("invalid repo group: %s", pr.RepoGroup)
    }

    status, err := client.GetCIStatus(ctx, parts[0], parts[1], pr.MergeCommitSHA)
    if err != nil {
        return false, err
    }

    return status == "success", nil
}

// getPRFromDB gets a PR from the database
func getPRFromDB(repoGroup, prID string) (*models.PRRecord, error) {
    data, err := db.Get(db.BucketPRs, repoGroup+"#"+prID)
    if err != nil {
        return nil, err
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        return nil, err
    }

    return &pr, nil
}

// splitRepoName splits a repo group name to get owner/repo
func splitRepoName(repoGroup string) []string {
    return []string{"owner", "repo"}
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
    for _, v := range slice {
        if v == value {
            return true
        }
    }
    return false
}
