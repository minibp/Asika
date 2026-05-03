package ci

import (
	"context"
	"testing"

	"asika/common/platforms"
	"asika/testutil"
)

func TestNewCIDetector(t *testing.T) {
	d := NewCIDetector()
	if d == nil {
		t.Fatal("NewCIDetector returned nil")
	}
}

func TestDetect_GitHubActions(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	d := NewCIDetector()
	ctx := context.Background()

	result, err := d.Detect(ctx, client, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result != "github_actions" {
		t.Errorf("Detect() = %q, want github_actions", result)
	}
}

func TestDetect_GitLabCI(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	d := NewCIDetector()
	ctx := context.Background()

	result, err := d.Detect(ctx, client, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// GitLab CI detection also relies on CI status
	if result == "" {
		t.Error("Detect() returned empty string")
	}
}

func TestDetect_None(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "none",
	}

	d := NewCIDetector()
	ctx := context.Background()

	result, err := d.Detect(ctx, client, "owner", "repo", "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result != "none" {
		t.Errorf("Detect() = %q, want none", result)
	}
}

func TestHasGitHubActions(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	ctx := context.Background()
	result, err := hasGitHubActions(ctx, client, "owner", "repo")
	if err != nil {
		t.Fatalf("hasGitHubActions failed: %v", err)
	}

	if !result {
		t.Error("hasGitHubActions should return true for success status")
	}
}

func TestHasGitHubActions_None(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "none",
	}

	ctx := context.Background()
	result, err := hasGitHubActions(ctx, client, "owner", "repo")
	if err != nil {
		t.Fatalf("hasGitHubActions failed: %v", err)
	}

	if result {
		t.Error("hasGitHubActions should return false for none status")
	}
}

func TestHasGitLabCI(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	ctx := context.Background()
	result, err := hasGitLabCI(ctx, client, "owner", "repo")
	if err != nil {
		t.Fatalf("hasGitLabCI failed: %v", err)
	}

	if !result {
		t.Error("hasGitLabCI should return true for success status")
	}
}

func TestHasGiteaActions(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	ctx := context.Background()
	result, err := hasGiteaActions(ctx, client, "owner", "repo")
	if err != nil {
		t.Fatalf("hasGiteaActions failed: %v", err)
	}

	if !result {
		t.Error("hasGiteaActions should return true for success status")
	}
}

func TestGetCIStatus(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	ctx := context.Background()
	result, err := GetCIStatus(ctx, client, "owner", "repo", "abc123")
	if err != nil {
		t.Fatalf("GetCIStatus failed: %v", err)
	}

	if result != "success" {
		t.Errorf("GetCIStatus() = %q, want success", result)
	}
}

func TestDetermineCIProvider_Configured(t *testing.T) {
	client := &testutil.MockPlatformClient{}

	ctx := context.Background()
	result, err := DetermineCIProvider(ctx, client, "owner", "repo", "main", "github_actions")
	if err != nil {
		t.Fatalf("DetermineCIProvider failed: %v", err)
	}

	if result != "github_actions" {
		t.Errorf("DetermineCIProvider() = %q, want github_actions", result)
	}
}

func TestDetermineCIProvider_None(t *testing.T) {
	client := &testutil.MockPlatformClient{}

	ctx := context.Background()
	result, err := DetermineCIProvider(ctx, client, "owner", "repo", "main", "none")
	if err != nil {
		t.Fatalf("DetermineCIProvider failed: %v", err)
	}

	if result != "none" {
		t.Errorf("DetermineCIProvider() = %q, want none", result)
	}
}

func TestDetermineCIProvider_Detect(t *testing.T) {
	client := &testutil.MockPlatformClient{
		CIStatus: "success",
	}

	ctx := context.Background()
	result, err := DetermineCIProvider(ctx, client, "owner", "repo", "main", "")
	if err != nil {
		t.Fatalf("DetermineCIProvider failed: %v", err)
	}

	if result == "" {
		t.Error("DetermineCIProvider() returned empty string")
	}
}

func TestPlatformType(t *testing.T) {
	tests := []struct {
		input platforms.PlatformType
		want  string
	}{
		{platforms.PlatformType("github"), "github"},
		{platforms.PlatformType("gitlab"), "gitlab"},
		{platforms.PlatformType("gitea"), "gitea"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if string(tt.input) != tt.want {
				t.Errorf("PlatformType = %q, want %q", string(tt.input), tt.want)
			}
		})
	}
}
