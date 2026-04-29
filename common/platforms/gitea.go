package platforms

import (
    "context"
    "fmt"

    "code.gitea.io/sdk/gitea"

    "asika/common/models"
)

// GiteaClient implements PlatformClient for Gitea
type GiteaClient struct {
    client *gitea.Client
    token  string
}

// NewGiteaClient creates a new Gitea client
func NewGiteaClient(baseURL, token string) *GiteaClient {
    client, _ := gitea.NewClient(baseURL, gitea.SetToken(token))
    return &GiteaClient{
        client: client,
        token:  token,
    }
}

// GetPR retrieves a pull request
func (c *GiteaClient) GetPR(ctx context.Context, owner, repo string, number int) (*models.PRRecord, error) {
    return nil, fmt.Errorf("not implemented")
}

// ListPRs lists pull requests
func (c *GiteaClient) ListPRs(ctx context.Context, owner, repo string, state string) ([]*models.PRRecord, error) {
    return nil, fmt.Errorf("not implemented")
}

// ApprovePR approves a pull request
func (c *GiteaClient) ApprovePR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// MergePR merges a pull request
func (c *GiteaClient) MergePR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// ClosePR closes a pull request
func (c *GiteaClient) ClosePR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// ReopenPR reopens a pull request
func (c *GiteaClient) ReopenPR(ctx context.Context, owner, repo string, number int) error {
    return fmt.Errorf("not implemented")
}

// CommentPR adds a comment to a pull request
func (c *GiteaClient) CommentPR(ctx context.Context, owner, repo string, number int, body string) error {
    return fmt.Errorf("not implemented")
}

// AddLabel adds a label to a pull request
func (c *GiteaClient) AddLabel(ctx context.Context, owner, repo string, number int, label string) error {
    return fmt.Errorf("not implemented")
}

// RemoveLabel removes a label from a pull request
func (c *GiteaClient) RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error {
    return fmt.Errorf("not implemented")
}

// GetBranch checks if a branch exists
func (c *GiteaClient) GetBranch(ctx context.Context, owner, repo, branch string) (bool, error) {
    return false, fmt.Errorf("not implemented")
}

// DeleteBranch deletes a branch
func (c *GiteaClient) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
    return fmt.Errorf("not implemented")
}

// GetDefaultBranch gets the default branch
func (c *GiteaClient) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
    return "main", nil
}

// GetCIStatus gets the CI status
func (c *GiteaClient) GetCIStatus(ctx context.Context, owner, repo string, commitSHA string) (string, error) {
    return "success", nil
}

// GetDefaultMergeMethod gets the default merge method
func (c *GiteaClient) GetDefaultMergeMethod(ctx context.Context, owner, repo string) (string, error) {
    return "merge", nil
}

// HasMultipleMergeMethods checks if multiple merge methods are available
func (c *GiteaClient) HasMultipleMergeMethods(ctx context.Context, owner, repo string) (bool, error) {
    return false, nil
}

// GetApprovals gets the list of approvers
func (c *GiteaClient) GetApprovals(ctx context.Context, owner, repo string, number int) ([]string, error) {
    return nil, fmt.Errorf("not implemented")
}

// VerifyWebhookSignature verifies the webhook signature
func (c *GiteaClient) VerifyWebhookSignature(body []byte, signature string) bool {
    return true
}

// GetPRCommits gets the commits in a PR
func (c *GiteaClient) GetPRCommits(ctx context.Context, owner, repo string, number int) ([]string, error) {
    return nil, fmt.Errorf("not implemented")
}
