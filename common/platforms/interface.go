package platforms

import (
    "context"

    "asika/common/models"
)

// PlatformType represents the type of platform
type PlatformType string

const (
    PlatformGitHub PlatformType = "github"
    PlatformGitLab PlatformType = "gitlab"
    PlatformGitea  PlatformType = "gitea"
)

// PlatformClient defines the unified platform operation interface
type PlatformClient interface {
    // PR operations
    GetPR(ctx context.Context, owner, repo string, number int) (*models.PRRecord, error)
    ListPRs(ctx context.Context, owner, repo string, state string) ([]*models.PRRecord, error)
    ApprovePR(ctx context.Context, owner, repo string, number int) error
    MergePR(ctx context.Context, owner, repo string, number int) error
    ClosePR(ctx context.Context, owner, repo string, number int) error
    ReopenPR(ctx context.Context, owner, repo string, number int) error
    CommentPR(ctx context.Context, owner, repo string, number int, body string) error
    AddLabel(ctx context.Context, owner, repo string, number int, label string) error
    RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error

    // Branch operations
    GetBranch(ctx context.Context, owner, repo, branch string) (bool, error)
    DeleteBranch(ctx context.Context, owner, repo, branch string) error
    GetDefaultBranch(ctx context.Context, owner, repo string) (string, error)

    // CI status
    GetCIStatus(ctx context.Context, owner, repo string, commitSHA string) (string, error)

    // Merge method
    GetDefaultMergeMethod(ctx context.Context, owner, repo string) (string, error)
    HasMultipleMergeMethods(ctx context.Context, owner, repo string) (bool, error)

    // Approval status
    GetApprovals(ctx context.Context, owner, repo string, number int) ([]string, error)

    // Webhook
    VerifyWebhookSignature(body []byte, signature string) bool

    // PR commits
    GetPRCommits(ctx context.Context, owner, repo string, number int) ([]string, error)
}
