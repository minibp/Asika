package syncer

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
    "asika/common/gitutil"
    "asika/common/platforms"
    "github.com/google/uuid"
    "github.com/go-git/go-git/v5"
)

// Syncer handles cross-platform synchronization
type Syncer struct {
    cfg     *models.Config
    clients map[platforms.PlatformType]platforms.PlatformClient
}

// NewSyncer creates a new syncer
func NewSyncer(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) *Syncer {
    return &Syncer{
        cfg:     cfg,
        clients: clients,
    }
}

// SyncOnMerge handles a merge event and syncs to other platforms
func (s *Syncer) SyncOnMerge(ctx context.Context, pr *models.PRRecord) error {
    if s.cfg.Mode != "multi" {
        slog.Info("skipping sync: not in multi mode")
        return nil
    }

    group := config.GetRepoGroupByName(s.cfg, pr.RepoGroup)
    if group == nil {
        return fmt.Errorf("repo group not found: %s", pr.RepoGroup)
    }

    // Create temp workdir
    workdir, err := gitutil.CreateTempWorkdir("asika-sync-")
    if err != nil {
        return fmt.Errorf("failed to create workdir: %w", err)
    }
    defer gitutil.CleanupWorkdir(workdir)

    // Setup repository
    repo, err := s.setupRepo(workdir, group, pr)
    if err != nil {
        return err
    }

    // Cherry-pick to other platforms
    if err := s.syncToOtherPlatforms(ctx, repo, workdir, group, pr); err != nil {
        return err
    }

    // Record sync history
    s.recordSync(pr, "success", "")

    return nil
}

// setupRepo sets up the git repository for syncing
func (s *Syncer) setupRepo(workdir string, group *models.RepoGroup, pr *models.PRRecord) (*git.Repository, error) {
    // Determine source platform and repo
    var sourceRepo string
    switch pr.Platform {
    case "github":
        sourceRepo = group.GitHub
    case "gitlab":
        sourceRepo = group.GitLab
    case "gitea":
        sourceRepo = group.Gitea
    default:
        return nil, fmt.Errorf("unknown platform: %s", pr.Platform)
    }

    // Clone or open repo
    token := s.getTokenForPlatform(pr.Platform)
    repo, err := gitutil.CloneOrOpen(workdir, s.getRepoURL(pr.Platform, sourceRepo), token)
    if err != nil {
        return nil, fmt.Errorf("failed to setup repo: %w", err)
    }

    return repo, nil
}

// syncToOtherPlatforms syncs the merge commit to other platforms
func (s *Syncer) syncToOtherPlatforms(ctx context.Context, repo *git.Repository, workdir string, group *models.RepoGroup, pr *models.PRRecord) error {
    platforms := []struct {
        name string
        repo string
    }{
        {"github", group.GitHub},
        {"gitlab", group.GitLab},
        {"gitea", group.Gitea},
    }

    for _, p := range platforms {
        if p.name == pr.Platform || p.repo == "" {
            continue
        }

        slog.Info("syncing to platform", "target", p.name, "repo", p.repo)

        // Cherry-pick
        if err := gitutil.CherryPick(repo, pr.MergeCommitSHA); err != nil {
            slog.Error("cherry-pick failed", "target", p.name, "error", err)
            s.recordSync(pr, "failed", fmt.Sprintf("cherry-pick to %s failed: %v", p.name, err))
            continue
        }

        // Push
        token := s.getTokenForPlatform(p.name)
        if err := gitutil.Push(repo, "origin", group.DefaultBranch, token); err != nil {
            slog.Error("push failed", "target", p.name, "error", err)
            s.recordSync(pr, "failed", fmt.Sprintf("push to %s failed: %v", p.name, err))
            continue
        }

        s.recordSync(pr, "success", "")
    }

    return nil
}

// recordSync records sync history
func (s *Syncer) recordSync(pr *models.PRRecord, status, errorMsg string) {
    record := models.SyncRecord{
        ID:             uuid.New().String(),
        RepoGroup:      pr.RepoGroup,
        SourcePlatform: pr.Platform,
        Branch:         "main",
        CommitSHA:      pr.MergeCommitSHA,
        Status:         status,
        ErrorMessage:   errorMsg,
        Timestamp:      time.Now(),
    }

    data, _ := json.Marshal(record)
    db.Put(db.BucketSyncHistory, record.ID, data)
}

// getTokenForPlatform gets the token for a platform
func (s *Syncer) getTokenForPlatform(platform string) string {
    switch platforms.PlatformType(platform) {
    case platforms.PlatformGitHub:
        return s.cfg.Tokens.GitHub
    case platforms.PlatformGitLab:
        return s.cfg.Tokens.GitLab
    case platforms.PlatformGitea:
        return s.cfg.Tokens.Gitea
    }
    return ""
}

// getRepoURL gets the clone URL for a platform
func (s *Syncer) getRepoURL(platform, repo string) string {
    parts := strings.Split(repo, "/")
    if len(parts) != 2 {
        return ""
    }

    switch platforms.PlatformType(platform) {
    case platforms.PlatformGitHub:
        return fmt.Sprintf("https://github.com/%s/%s.git", parts[0], parts[1])
    case platforms.PlatformGitLab:
        return fmt.Sprintf("https://gitlab.com/%s/%s.git", parts[0], parts[1])
    case platforms.PlatformGitea:
        return fmt.Sprintf("https://gitea.example.com/%s/%s.git", parts[0], parts[1])
    }
    return ""
}
