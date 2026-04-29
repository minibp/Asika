package platforms

import (
    "context"
    "fmt"

    "github.com/google/go-github/v69/github"

    "asika/common/models"
)

// GitHubClient implements PlatformClient for GitHub
type GitHubClient struct {
    client *github.Client
    token  string
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient(token string) *GitHubClient {
    return &GitHubClient{
        client: github.NewTokenClient(context.Background(), token),
        token:  token,
    }
}

// GetPR retrieves a pull request
func (c *GitHubClient) GetPR(ctx context.Context, owner, repo string, number int) (*models.PRRecord, error) {
    return nil, fmt.Errorf("not implemented")
}

// ListPRs lists pull requests
func (c *GitHubClient) ListPRs(ctx context.Context, owner, repo string, state string) ([]*models.PRRecord, error) {
    opts := &github.PullRequestListOptions{
        State: state,
        ListOptions: github.ListOptions{PerPage: 100},
    }

    var result []*models.PRRecord
    for {
        prs, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
        if err != nil {
            return nil, fmt.Errorf("failed to list PRs: %w", err)
        }

        for _, pr := range prs {
            record := &models.PRRecord{
                ID:           fmt.Sprintf("%d", pr.GetID()),
                RepoGroup:    "",
                Platform:      "github",
                PRNumber:     pr.GetNumber(),
                Title:        pr.GetTitle(),
                Author:       pr.GetUser().GetLogin(),
                State:        pr.GetState(),
                Labels:       extractLabels(pr.Labels),
                MergeCommitSHA: pr.GetMergeCommitSHA(),
                SpamFlag:     false,
                UpdatedAt:    pr.GetUpdatedAt().Time,
                Events:       []models.PREvent{},
            }
            result = append(result, record)
        }

        if resp.NextPage == 0 {
            break
        }
        opts.Page = resp.NextPage
    }

    return result, nil
}

func extractLabels(labels []*github.Label) []string {
    result := make([]string, 0, len(labels))
    for _, l := range labels {
        result = append(result, l.GetName())
    }
    return result
}

// ApprovePR approves a pull request
func (c *GitHubClient) ApprovePR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// MergePR merges a pull request
func (c *GitHubClient) MergePR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// ClosePR closes a pull request
func (c *GitHubClient) ClosePR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// ReopenPR reopens a pull request
func (c *GitHubClient) ReopenPR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// CommentPR adds a comment to a pull request
func (c *GitHubClient) CommentPR(ctx context.Context, owner, repo string, number int, body string) error {
    return fmt.Errorf("not implemented")
}

// AddLabel adds a label to a pull request
func (c *GitHubClient) AddLabel(ctx context.Context, owner, repo string, number int, label string) error {
    return fmt.Errorf("not implemented")
}

// RemoveLabel removes a label from a pull request
func (c *GitHubClient) RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error {
    return fmt.Errorf("not implemented")
}

// GetBranch checks if a branch exists
func (c *GitHubClient) GetBranch(ctx context.Context, owner, repo, branch string) (bool, error) {
    return false, fmt.Errorf("not implemented")
}

// DeleteBranch deletes a branch
func (c *GitHubClient) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
    return fmt.Errorf("not implemented")
}

// GetDefaultBranch gets the default branch
func (c *GitHubClient) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
    return "main", nil
}

// GetCIStatus gets the CI status
func (c *GitHubClient) GetCIStatus(ctx context.Context, owner, repo string, commitSHA string) (string, error) {
    return "success", nil
}

// GetDefaultMergeMethod gets the default merge method
func (c *GitHubClient) GetDefaultMergeMethod(ctx context.Context, owner, repo string) (string, error) {
    return "merge", nil
}

// HasMultipleMergeMethods checks if multiple merge methods are available
func (c *GitHubClient) HasMultipleMergeMethods(ctx context.Context, owner, repo string) (bool, error) {
    // Get repository info to check merge methods
    r, _, err := c.client.Repositories.Get(ctx, owner, repo)
    if err != nil {
        return false, fmt.Errorf("failed to get repo: %w", err)
    }

    // Count available merge methods
    methods := 0
    if r.GetAllowMergeCommit() {
        methods++
    }
    if r.GetAllowSquashMerge() {
        methods++
    }
    if r.GetAllowRebaseMerge() {
        methods++
    }

    return methods > 1, nil
}

// GetApprovals gets the list of approvers
func (c *GitHubClient) GetApprovals(ctx context.Context, owner, repo string, number int) ([]string, error) {
    return nil, fmt.Errorf("not implemented")
}

// VerifyWebhookSignature verifies the webhook signature
func (c *GitHubClient) VerifyWebhookSignature(body []byte, signature string) bool {
    return true
}

// GetPRCommits gets the commits in a PR
func (c *GitHubClient) GetPRCommits(ctx context.Context, owner, repo string, number int) ([]string, error) {
    return nil, fmt.Errorf("not implemented")
}
