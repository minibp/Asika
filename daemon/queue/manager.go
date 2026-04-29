package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "time"

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
    item := models.QueueItem{
        PRID:      pr.ID,
        RepoGroup: pr.RepoGroup,
        Status:    "waiting",
        AddedAt:   time.Now(),
    }

    data, err := json.Marshal(item)
    if err != nil {
        return err
    }

    key := pr.RepoGroup + "#" + pr.ID
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

        // Run checks
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

        // Save updated item
        updated, _ := json.Marshal(item)
        return db.Put(db.BucketQueueItems, string(key), updated)
    })

    if err != nil {
        slog.Error("failed to check queue", "error", err)
    }
}

// merge performs the merge
func (m *Manager) merge(item *models.QueueItem) error {
    ctx := context.Background()

    // Get PR details
    prData, err := db.Get(db.BucketPRs, item.RepoGroup+"#"+item.PRID)
    if err != nil {
        return err
    }

    var pr models.PRRecord
    if err := json.Unmarshal(prData, &pr); err != nil {
        return err
    }

    // Get platform client
    client := m.clients[platforms.PlatformType(pr.Platform)]
    if client == nil {
        return fmt.Errorf("no client for platform: %s", pr.Platform)
    }

    // Parse repo
    parts := splitRepo(pr.RepoGroup)
    if len(parts) != 2 {
        return fmt.Errorf("invalid repo group: %s", pr.RepoGroup)
    }

    // Merge PR
    return client.MergePR(ctx, parts[0], parts[1], pr.PRNumber)
}

// splitRepo splits repo group name to get owner/repo
func splitRepo(repoGroup string) []string {
    // This is simplified - need to get actual repo from config
    return []string{"owner", "repo"}
}
