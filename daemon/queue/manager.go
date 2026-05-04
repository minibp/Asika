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
	// First, read all items and collect done keys for cleanup
	var items []models.QueueItem
	var keys []string
	var doneKeys []string
	err := db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
		var item models.QueueItem
		if err := json.Unmarshal(value, &item); err != nil {
			return err
		}
		// Collect completed items for cleanup
		if item.Status == "done" {
			doneKeys = append(doneKeys, string(key))
			return nil
		}
		// Process waiting, checking, and failed items (failed items can be retried)
		if item.Status != "waiting" && item.Status != "checking" && item.Status != "failed" {
			return nil
		}
		items = append(items, item)
		keys = append(keys, string(key))
		return nil
	})
	if err != nil {
		slog.Error("failed to read queue items", "error", err)
		return
	}

	// Clean up completed items outside the read transaction
	for _, dk := range doneKeys {
		slog.Info("removing completed item from queue", "pr_id", dk)
		if delErr := db.Delete(db.BucketQueueItems, dk); delErr != nil {
			slog.Error("failed to remove completed queue item", "error", delErr)
		}
	}

	// Process items outside any db transaction
	for i, item := range items {
		item.Status = "checking"
		item.LastChecked = time.Now()

		shouldMerge, err := m.checker.ShouldMerge(&item)
		if err != nil {
			if isTransientError(err) {
				slog.Warn("transient check error, keeping as waiting", "error", err, "pr_id", item.PRID)
				item.Status = "waiting"
			} else {
				slog.Error("check failed", "error", err, "pr_id", item.PRID)
				item.Status = "failed"
				item.FailureReason = err.Error()
			}
			updated, _ := json.Marshal(item)
			if putErr := db.Put(db.BucketQueueItems, keys[i], updated); putErr != nil {
				slog.Error("failed to update queue item", "error", putErr, "pr_id", item.PRID)
			}
		} else if shouldMerge {
			item.Status = "merging"
			if err := m.merge(&item); err != nil {
				item.Status = "failed"
				item.FailureReason = err.Error()
				updated, _ := json.Marshal(item)
				if putErr := db.Put(db.BucketQueueItems, keys[i], updated); putErr != nil {
					slog.Error("failed to update queue item", "error", putErr, "pr_id", item.PRID)
				}
			} else {
				item.Status = "done"
				slog.Info("removing completed item from queue", "pr_id", item.PRID)
				if delErr := db.Delete(db.BucketQueueItems, keys[i]); delErr != nil {
					slog.Error("failed to remove completed queue item", "error", delErr, "pr_id", item.PRID)
				}
			}
		} else {
			item.Status = "waiting"
			updated, _ := json.Marshal(item)
			if putErr := db.Put(db.BucketQueueItems, keys[i], updated); putErr != nil {
				slog.Error("failed to update queue item", "error", putErr, "pr_id", item.PRID)
			}
		}
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
	method, err := client.GetDefaultMergeMethod(ctx, owner, repo)
	if err != nil {
		slog.Warn("failed to get default merge method, using default", "error", err)
		method = "merge"
	}

	slog.Info("merging PR", "pr_id", pr.ID, "pr_number", pr.PRNumber, "platform", pr.Platform, "method", method)
	err = client.MergePR(ctx, owner, repo, pr.PRNumber, method)
	if err != nil {
		slog.Error("merge failed", "pr_id", pr.ID, "error", err)
	} else {
		slog.Info("merge succeeded", "pr_id", pr.ID)
		// Fetch updated PR info from platform to get merge_commit_sha
		updated, getErr := client.GetPR(ctx, owner, repo, pr.PRNumber)
		if getErr == nil && updated != nil {
			pr.State = updated.State
			pr.MergeCommitSHA = updated.MergeCommitSHA
			pr.UpdatedAt = updated.UpdatedAt
		} else {
			pr.State = "merged"
			pr.UpdatedAt = time.Now()
		}
		key := fmt.Sprintf("%s#%s#%d", pr.RepoGroup, pr.Platform, pr.PRNumber)
		data, _ := json.Marshal(pr)
		db.PutPRWithIndex(key, data, pr.ID, pr.RepoGroup, pr.PRNumber)
	}
	return err
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
		if item.RepoGroup == repoGroup || strings.HasPrefix(string(key), repoGroup+"#") {
			items = append(items, item)
		}
		return nil
	})
	return items, err
}