package models

import "time"

// User represents an admin user
type User struct {
    Username     string    `json:"username"`
    PasswordHash string    `json:"password_hash"` // bcrypt
    Role         string    `json:"role"`          // "admin" | "operator" | "viewer"
    CreatedAt    time.Time `json:"created_at"`
}

// RepoGroup represents a repository group
type RepoGroup struct {
    Name           string          `json:"name"`
    Mode           string          `json:"mode"`            // "multi" | "single"
    MirrorPlatform string          `json:"mirror_platform"` // single mode source platform, e.g. "github"
    GitHub         string          `json:"github"`
    GitLab         string          `json:"gitlab"`
    Gitea          string          `json:"gitea"`
    DefaultBranch  string          `json:"default_branch"`
    HookPath       string          `json:"hookpath"`
    CIProvider     string          `json:"ci_provider"`
    MergeQueue     MergeQueueConfig `json:"merge_queue"`
}

// PRRecord represents a pull request record
type PRRecord struct {
    ID             string    `json:"id"` // UUID
    RepoGroup      string    `json:"repo_group"`
    Platform       string    `json:"platform"` // "github"|"gitlab"|"gitea"
    PRNumber       int       `json:"pr_number"`
    Title          string    `json:"title"`
    Author         string    `json:"author"`
    State          string    `json:"state"` // "open"|"closed"|"merged"|"spam"
    Labels         []string  `json:"labels"`
    MergeCommitSHA string    `json:"merge_commit_sha"`
    SpamFlag       bool      `json:"spam_flag"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
    DiffFiles      []string  `json:"diff_files"` // changed file list for label rules
    Events         []PREvent `json:"events"`
}

// PREvent represents a pull request event
type PREvent struct {
    Timestamp time.Time `json:"timestamp"`
    Action    string    `json:"action"` // "opened"|"closed"|"merged"|"approved"|"label_added"|"synced"|"cherry_picked"|"comment"|...
    Actor     string    `json:"actor"`
    Detail    string    `json:"detail"`
}

// QueueItem represents a merge queue item
type QueueItem struct {
    PRID          string         `json:"pr_id"`
    RepoGroup     string         `json:"repo_group"`
    Status        string         `json:"status"` // "waiting"|"checking"|"merging"|"done"|"failed"
    AddedAt       time.Time      `json:"added_at"`
    LastChecked   time.Time      `json:"last_checked"`
    FailureReason string         `json:"failure_reason,omitempty"`
    Criteria      MergeCriteria  `json:"criteria"`
}

// MergeCriteria represents a snapshot of merge conditions
type MergeCriteria struct {
    RequiredApprovals int      `json:"required_approvals"`
    ApprovedBy        []string `json:"approved_by"`
    CIStatus          string   `json:"ci_status"` // "pending"|"success"|"failure"|"none"
}

// AuditLog represents an audit log entry
type AuditLog struct {
    Timestamp time.Time              `json:"timestamp"`
    Level     string                 `json:"level"` // "info"|"warn"|"error"
    Message   string                 `json:"message"`
    Context   map[string]interface{} `json:"context,omitempty"`
}

// SyncRecord represents a sync history record
type SyncRecord struct {
    ID             string    `json:"id"`
    RepoGroup      string    `json:"repo_group"`
    SourcePlatform string    `json:"source_platform"`
    TargetPlatform string    `json:"target_platform"`
    Branch         string    `json:"branch"`
    CommitSHA      string    `json:"commit_sha"`
    Status         string    `json:"status"` // "success"|"failed"
    ErrorMessage   string    `json:"error_message,omitempty"`
    Timestamp      time.Time `json:"timestamp"`
}

// MergeQueueConfig represents merge queue configuration
type MergeQueueConfig struct {
    RequiredApprovals int      `json:"required_approvals" toml:"required_approvals"`
    CICheckRequired   bool     `json:"ci_check_required" toml:"ci_check_required"`
    CoreContributors  []string `json:"core_contributors" toml:"core_contributors"`
    CIProvider        string   `json:"ci_provider" toml:"ci_provider"` // per-repo-group override
}

// LabelRule represents a label rule
type LabelRule struct {
    Pattern string `json:"pattern" toml:"pattern"`
    Label   string `json:"label" toml:"label"`
}

// SpamConfig represents spam detection configuration
type SpamConfig struct {
    Enabled                   bool     `json:"enabled" toml:"enabled"`
    TimeWindow                string   `json:"time_window" toml:"time_window"`
    Threshold                 int      `json:"threshold" toml:"threshold"`
    TriggerOnAuthor           bool     `json:"trigger_on_author" toml:"trigger_on_author"`
    TriggerOnSimilarTitle     bool     `json:"trigger_on_similar_title" toml:"trigger_on_similar_title"`
    TitleSimilarityThreshold  float64  `json:"title_similarity_threshold" toml:"title_similarity_threshold"`
    TriggerOnKeywords         []string `json:"trigger_on_keywords" toml:"trigger_on_title_kw"`
}

// NotifyConfig represents notification configuration
type NotifyConfig struct {
    Type   string                 `toml:"type"`
    Config map[string]interface{} `toml:"config"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
    Listen string `toml:"listen"`
    Mode   string `toml:"mode"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
    Path string `toml:"path"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
    JWTSecret   string `toml:"jwt_secret"`
    TokenExpiry string `toml:"token_expiry"`
}

// EventsConfig represents events configuration
type EventsConfig struct {
    Mode            string `toml:"mode"`
    WebhookSecret   string `toml:"webhook_secret"`
    PollingInterval string `toml:"polling_interval"`
}

// GitConfig represents git configuration
type GitConfig struct {
    WorkDir string `toml:"workdir"`
}

// TokensConfig represents platform token configuration
type TokensConfig struct {
    GitHub string `toml:"github"`
    GitLab string `toml:"gitlab"`
    Gitea  string `toml:"gitea"`
}

// RepoGroupConfig represents repository group configuration (TOML mapping)
type RepoGroupConfig struct {
    Name          string          `toml:"name"`
    Mode          string          `toml:"mode"` // "multi" | "single", overrides global
    GitHub        string          `toml:"github"`
    GitLab        string          `toml:"gitlab"`
    Gitea         string          `toml:"gitea"`
    DefaultBranch string          `toml:"default_branch"`
    HookPath      string          `toml:"hookpath"`
    CIProvider    string          `toml:"ci_provider"`
    MergeQueue    MergeQueueConfig `toml:"merge_queue"`
}

// SingleRepoConfig represents single repository configuration
type SingleRepoConfig struct {
    Platform      string `toml:"platform"` // mirror_platform in tasks.md, "github"|"gitlab"|"gitea"
    Repo          string `toml:"repo"`
    DefaultBranch string `toml:"default_branch"`
    HookPath      string `toml:"hookpath"`
    CIProvider    string `toml:"ci_provider"`
}

// Config represents the main configuration structure
type Config struct {
    Server      ServerConfig      `toml:"server"`
    Mode        string            `toml:"mode"` // "single" | "multi"
    Database    DatabaseConfig    `toml:"database"`
    Auth        AuthConfig        `toml:"auth"`
    Notify      []NotifyConfig    `toml:"notify"`
    Events      EventsConfig      `toml:"events"`
    Git         GitConfig         `toml:"git"`
    Tokens      TokensConfig      `toml:"tokens"`
    LabelRules  []LabelRule       `toml:"label_rules"`
    Spam        SpamConfig        `toml:"spam"`
    MergeQueue  MergeQueueConfig  `toml:"merge_queue"`
    HookPath    string            `toml:"hookpath"`
    RepoGroups  []RepoGroupConfig `toml:"repo_groups"`
    SingleRepo  SingleRepoConfig  `toml:"single_repo"`
}