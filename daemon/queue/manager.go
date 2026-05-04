package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/platforms"
)

// Manager manages the merge queue
type Manager struct {
	cfg     *models.Config
	clients map[platforms.PlatformType]platforms.PlatformClient
	checker *Checker
}

// NewManager creates a new queue manager
func NewManager(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) *Manager {
	return &Manager{
		cfg:     cfg,
		clients: clients,
		checker: NewChecker(cfg, clients),
	}
}

// AddToQueue adds a PR to the merge queue
func (m *Manager) AddToQueue(pr *models.PRRecord) error {
	key := fmt.Sprintf("%s#%s", pr.RepoGroup, pr.ID)

	// Check if already in queue
	existing, err := db.Get(db.BucketQueueItems, key)
	if err == nil && existing != nil {
		slog.Info("PR already in queue", "pr_id", pr.ID)
		return nil
	}

	// Skip draft PRs
	if pr.IsDraft {
		slog.Info("skipping draft PR", "pr_id", pr.ID, "title", pr.Title)
		return nil
	}

	// Get merge criteria from repo group config
	group := config.GetRepoGroupByName(m.cfg, pr.RepoGroup)
	criteria := models.MergeCriteria{
		RequiredApprovals: 1,
		CIStatus:          "pending",
	}
	if group != nil {
		criteria.RequiredApprovals = group.MergeQueue.RequiredApprovals
	}

	item := models.QueueItem{
		PRID:      pr.ID,
		RepoGroup: pr.RepoGroup,
		Status:    "waiting",
		AddedAt:   time.Now(),
		Criteria:  criteria,
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	slog.Info("PR added to merge queue", "pr_id", pr.ID, "repo_group", pr.RepoGroup)
	return db.Put(db.BucketQueueItems, key, data)
}

// CheckQueue checks all items in the queue
func (m *Manager) CheckQueue() {
	err := db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
		var item models.QueueItem
		if err := json.Unmarshal(value, &item); err != nil {
			return err
		}

		if item.Status != "waiting" && item.Status != "checking" {
			return nil
		}

		// Update status to checking
		item.Status = "checking"
		item.LastChecked = time.Now()

		shouldMerge, err := m.checker.ShouldMerge(&item)
		if err != nil {
			slog.Error("check failed", "error", err, "pr_id", item.PRID)
			item.Status = "failed"
			item.FailureReason = err.Error()
		} else if shouldMerge {
			item.Status = "merging"
			if err := m.merge(&item); err != nil {
				item.Status = "failed"
				item.FailureReason = err.Error()
			} else {
				item.Status = "done"
			}
		} else {
			item.Status = "waiting"
		}

		updated, _ := json.Marshal(item)
		return db.Put(db.BucketQueueItems, string(key), updated)
	})

	if err != nil {
		slog.Error("failed to check queue", "error", err)
	}
}

// merge performs the merge operation
func (m *Manager) merge(item *models.QueueItem) error {
	ctx := context.Background()

	// Find PR in bbolt
	pr, err := findPRByID(item.PRID)
	if err != nil {
		return err
	}

	// Get platform client
	client := m.clients[platforms.PlatformType(pr.Platform)]
	if client == nil {
		return fmt.Errorf("no client for platform: %s", pr.Platform)
	}

	// Get repo group config
	group := config.GetRepoGroupByName(m.cfg, pr.RepoGroup)
	if group == nil {
		return fmt.Errorf("repo group not found: %s", pr.RepoGroup)
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		return fmt.Errorf("cannot resolve repo for platform %s", pr.Platform)
	}

	// Determine merge method
	method := ""
	hasMultiple, err := client.HasMultipleMergeMethods(ctx, owner, repo)
	if err == nil && hasMultiple {
		method, _ = client.GetDefaultMergeMethod(ctx, owner, repo)
	}

	slog.Info("merging PR", "pr_id", pr.ID, "platform", pr.Platform, "method", method)
	return client.MergePR(ctx, owner, repo, pr.PRNumber, method)
}

// findPRByID finds a PR by its ID in bbolt
func findPRByID(prID string) (*models.PRRecord, error) {
    data, err := db.GetPRByIndex(prID, "", 0)
    if err != nil || data == nil {
        var pr *models.PRRecord
        err := db.ForEach(db.BucketPRs, func(key, value []byte) error {
            var record models.PRRecord
            if err := json.Unmarshal(value, &record); err != nil {
                return err
            }
            if record.ID == prID {
                pr = &record
            }
            return nil
        })
        if err != nil {
            return nil, err
        }
        if pr == nil {
            return nil, fmt.Errorf("PR not found: %s", prID)
        }
        return pr, nil
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        return nil, err
    }
    return &pr, nil
}

// GetQueueItems returns all queue items for a repo group
func (m *Manager) GetQueueItems(repoGroup string) ([]models.QueueItem, error) {
	var items []models.QueueItem
	err := db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
		var item models.QueueItem
		if err := json.Unmarshal(value, &item); err != nil {
			return err
		}
		if strings.HasPrefix(string(key), repoGroup+"#") || strings.HasPrefix(string(key), repoGroup) {
			items = append(items, item)
		}
		return nil
	})
	return items, err
}