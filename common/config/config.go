package config

import (
    "fmt"
    "log/slog"
    "os"
    "strings"
    "sync/atomic"
    "time"

    "github.com/BurntSushi/toml"
    "github.com/google/uuid"

    "asika/common/models"
)

var (
    current atomic.Value
    ConfigPath string
)

// Store stores the configuration atomically
func Store(cfg *models.Config) {
    current.Store(cfg)
}

// Current returns the current configuration
func Current() *models.Config {
    v := current.Load()
    if v == nil {
        return nil
    }
    return v.(*models.Config)
}

// Load loads configuration from the TOML file
func Load(path string) (*models.Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    cfg := &models.Config{
        Mode: "multi",
        Server: models.ServerConfig{
            Listen: ":8080",
            Mode:   "release",
        },
        MergeQueue: models.MergeQueueConfig{
            RequiredApprovals: 1,
            CICheckRequired:   true,
        },
    }

    if err := toml.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %w", err)
    }

    // Apply environment variable overrides for tokens
    if token := os.Getenv("ASIKA_GITHUB_TOKEN"); token != "" {
        cfg.Tokens.GitHub = token
    }
    if token := os.Getenv("ASIKA_GITLAB_TOKEN"); token != "" {
        cfg.Tokens.GitLab = token
    }
    if token := os.Getenv("ASIKA_GITEA_TOKEN"); token != "" {
        cfg.Tokens.Gitea = token
    }

    // Validate configuration
    if err := validate(cfg); err != nil {
        return nil, err
    }

    Store(cfg)
    ConfigPath = path
    return cfg, nil
}

// validate validates the configuration
func validate(cfg *models.Config) error {
    if cfg.Mode != "single" && cfg.Mode != "multi" {
        return fmt.Errorf("invalid mode: %s (must be 'single' or 'multi')", cfg.Mode)
    }

    if cfg.Mode == "multi" && len(cfg.RepoGroups) == 0 {
        return fmt.Errorf("multi mode requires at least one repo_groups entry")
    }

    if cfg.Mode == "single" {
        if cfg.SingleRepo.Repo == "" {
            return fmt.Errorf("single mode requires single_repo.repo to be set")
        }
        if cfg.SingleRepo.Platform == "" {
            return fmt.Errorf("single mode requires single_repo.platform to be set")
        }
    }

    if cfg.Database.Path == "" {
        return fmt.Errorf("database.path is required")
    }

    if cfg.Auth.JWTSecret == "" {
        return fmt.Errorf("auth.jwt_secret is required")
    }

    return nil
}

// GetRepoGroups returns all repo groups based on mode
func GetRepoGroups(cfg *models.Config) []models.RepoGroup {
    if cfg.Mode == "single" {
        return []models.RepoGroup{
            {
                Name:           "default",
                Mode:           "single",
                MirrorPlatform: cfg.SingleRepo.Platform,
                GitHub:         cfg.SingleRepo.Repo,
                GitLab:         cfg.SingleRepo.Repo,
                Gitea:          cfg.SingleRepo.Repo,
                DefaultBranch:  cfg.SingleRepo.DefaultBranch,
                HookPath:       cfg.SingleRepo.HookPath,
                CIProvider:     cfg.SingleRepo.CIProvider,
                MergeQueue:     cfg.MergeQueue,
            },
        }
    }

    groups := make([]models.RepoGroup, len(cfg.RepoGroups))
    for i, rg := range cfg.RepoGroups {
        mode := rg.Mode
        if mode == "" {
            mode = cfg.Mode
        }
        groups[i] = models.RepoGroup{
            Name:           rg.Name,
            Mode:           mode,
            MirrorPlatform: "",
            GitHub:         rg.GitHub,
            GitLab:         rg.GitLab,
            Gitea:          rg.Gitea,
            DefaultBranch:  rg.DefaultBranch,
            HookPath:       rg.HookPath,
            CIProvider:     rg.CIProvider,
            MergeQueue:     rg.MergeQueue,
        }
    }
    return groups
}

// GetRepoGroupByName finds a repo group by name
func GetRepoGroupByName(cfg *models.Config, name string) *models.RepoGroup {
    groups := GetRepoGroups(cfg)
    for _, g := range groups {
        if g.Name == name {
            return &g
        }
    }
    return nil
}

// GetOwnerRepoFromGroup returns the owner/repo for a platform in a repo group
func GetOwnerRepoFromGroup(group *models.RepoGroup, platform string) (owner, repo string) {
    var repoPath string
    switch platform {
    case "github":
        repoPath = group.GitHub
    case "gitlab":
        repoPath = group.GitLab
    case "gitea":
        repoPath = group.Gitea
    }
    if repoPath == "" {
        return "", ""
    }
    idx := strings.LastIndex(repoPath, "/")
    if idx < 0 {
        return "", repoPath
    }
    return repoPath[:idx], repoPath[idx+1:]
}

// GenerateTokenExpiry parses the token expiry duration
func GenerateTokenExpiry(expiry string) time.Duration {
    d, err := time.ParseDuration(expiry)
    if err != nil {
        slog.Warn("invalid token_expiry, using default 72h", "error", err)
        d = 72 * time.Hour
    }
    return d
}

// GenerateUUID generates a new UUID string
func GenerateUUID() string {
    return uuid.New().String()
}