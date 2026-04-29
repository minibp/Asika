package syncer

import (
    "context"
    "encoding/json"
    "log/slog"
    "strings"
    "time"

    "asika/common/db"
    "asika/common/models"
)

// SpamDetector detects and handles spam PRs
type SpamDetector struct {
    cfg *models.Config
}

// NewSpamDetector creates a new spam detector
func NewSpamDetector(cfg *models.Config) *SpamDetector {
    return &SpamDetector{
        cfg: cfg,
    }
}

// Scan scans for spam PRs
func (d *SpamDetector) Scan() {
    if !d.cfg.Spam.Enabled {
        return
    }

    ctx := context.Background()
    window := parseDuration(d.cfg.Spam.TimeWindow, 10*time.Minute)
    cutoff := time.Now().Add(-window)

    // Get PRs within time window
    prs := d.getPRsAfter(cutoff)

    // Check for spam
    spamPRs := d.detectSpam(prs)

    // Handle spam PRs
    for _, pr := range spamPRs {
        d.handleSpam(ctx, pr)
    }
}

// getPRsAfter gets PRs created after a certain time
func (d *SpamDetector) getPRsAfter(after time.Time) []*models.PRRecord {
    prs := make([]*models.PRRecord, 0)

    db.ForEach(db.BucketPRs, func(key, value []byte) error {
        var pr models.PRRecord
        if err := json.Unmarshal(value, &pr); err != nil {
            return err
        }

        if pr.CreatedAt.After(after) {
            prs = append(prs, &pr)
        }

        return nil
    })

    return prs
}

// detectSpam detects spam based on configured rules
func (d *SpamDetector) detectSpam(prs []*models.PRRecord) []*models.PRRecord {
    spamMap := make(map[string]*models.PRRecord)

    // Check by author
    if d.cfg.Spam.TriggerOnAuthor {
        authorCount := make(map[string][]*models.PRRecord)
        for _, pr := range prs {
            authorCount[pr.Author] = append(authorCount[pr.Author], pr)
        }

        for _, prList := range authorCount {
            if len(prList) >= d.cfg.Spam.Threshold {
                for _, pr := range prList {
                    spamMap[pr.ID] = pr
                }
            }
        }
    }

    // Check by similar title
    if d.cfg.Spam.TriggerOnSimilarTitle {
        for i := 0; i < len(prs); i++ {
            for j := i + 1; j < len(prs); j++ {
                if d.isSimilarTitle(prs[i].Title, prs[j].Title) {
                    spamMap[prs[i].ID] = prs[i]
                    spamMap[prs[j].ID] = prs[j]
                }
            }
        }
    }

    // Check by keywords
    keywords := d.cfg.Spam.TriggerOnKeywords
    for _, pr := range prs {
        for _, keyword := range keywords {
            if strings.Contains(strings.ToLower(pr.Title), strings.ToLower(keyword)) {
                spamMap[pr.ID] = pr
                break
            }
        }
    }

    result := make([]*models.PRRecord, 0, len(spamMap))
    for _, pr := range spamMap {
        result = append(result, pr)
    }

    return result
}

// isSimilarTitle checks if two titles are similar
func (d *SpamDetector) isSimilarTitle(title1, title2 string) bool {
    // Simple similarity check - can be improved
    return strings.Contains(strings.ToLower(title1), strings.ToLower(title2)) ||
        strings.Contains(strings.ToLower(title2), strings.ToLower(title1))
}

// handleSpam handles a spam PR
func (d *SpamDetector) handleSpam(ctx context.Context, pr *models.PRRecord) {
    // Mark as spam
    pr.SpamFlag = true
    pr.State = "spam"
    pr.UpdatedAt = time.Now()

    data, _ := json.Marshal(pr)
    db.Put(db.BucketPRs, pr.RepoGroup+"#"+pr.ID, data)

    // Close PR via platform API
    // This would need platform client

    // Send notifications
    // This would use the notifier

    slog.Warn("spam detected", "pr_id", pr.ID, "author", pr.Author, "title", pr.Title)
}

// parseDuration parses a duration string
func parseDuration(s string, defaultDur time.Duration) time.Duration {
    d, err := time.ParseDuration(s)
    if err != nil {
        return defaultDur
    }
    return d
}
