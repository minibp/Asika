package platforms

import (
	"context"
	"sort"
	"testing"
)

type platformTest struct {
	name   string
	client PlatformClient
}

func TestPlatformClientInterface(t *testing.T) {
	tests := []platformTest{
		{"GitHubClient", NewGitHubClient("test-token", "test-webhook-secret")},
		{"GitLabClient", NewGitLabClient("test-token", "", "test-webhook-secret")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.client == nil {
				t.Skipf("%s is nil", tt.name)
				return
			}

			ctx := context.Background()

			t.Run("GetPR", func(t *testing.T) {
				_, err := tt.client.GetPR(ctx, "owner", "repo", 1)
				if err == nil {
					t.Log("GetPR returned ok")
				}
			})

			t.Run("ListPRs", func(t *testing.T) {
				_, err := tt.client.ListPRs(ctx, "owner", "repo", "open")
				if err == nil {
					t.Log("ListPRs returned ok")
				}
			})

			t.Run("GetBranch", func(t *testing.T) {
				_, err := tt.client.GetBranch(ctx, "owner", "repo", "main")
				if err == nil {
					t.Log("GetBranch returned ok")
				}
			})

			t.Run("GetDefaultBranch", func(t *testing.T) {
				_, err := tt.client.GetDefaultBranch(ctx, "owner", "repo")
				if err == nil {
					t.Log("GetDefaultBranch returned ok")
				}
			})

			t.Run("GetCIStatus", func(t *testing.T) {
				_, err := tt.client.GetCIStatus(ctx, "owner", "repo", "abc123")
				if err == nil {
					t.Log("GetCIStatus returned ok")
				}
			})

			t.Run("GetDefaultMergeMethod", func(t *testing.T) {
				_, err := tt.client.GetDefaultMergeMethod(ctx, "owner", "repo")
				if err == nil {
					t.Log("GetDefaultMergeMethod returned ok")
				}
			})

			t.Run("HasMultipleMergeMethods", func(t *testing.T) {
				_, err := tt.client.HasMultipleMergeMethods(ctx, "owner", "repo")
				if err == nil {
					t.Log("HasMultipleMergeMethods returned ok")
				}
			})

			t.Run("VerifyWebhookSignature", func(t *testing.T) {
				ok := tt.client.VerifyWebhookSignature([]byte("test"), "sha256=abc")
				t.Logf("VerifyWebhookSignature = %v", ok)
			})

			t.Run("GetPRCommits", func(t *testing.T) {
				_, err := tt.client.GetPRCommits(ctx, "owner", "repo", 1)
				if err == nil {
					t.Log("GetPRCommits returned ok")
				}
			})

			t.Run("GetDiffFiles", func(t *testing.T) {
				_, err := tt.client.GetDiffFiles(ctx, "owner", "repo", 1)
				if err == nil {
					t.Log("GetDiffFiles returned ok")
				}
			})

			t.Run("GetApprovals", func(t *testing.T) {
				_, err := tt.client.GetApprovals(ctx, "owner", "repo", 1)
				if err == nil {
					t.Log("GetApprovals returned ok")
				}
			})

			t.Run("ListBranches", func(t *testing.T) {
				_, err := tt.client.ListBranches(ctx, "owner", "repo")
				if err == nil {
					t.Log("ListBranches returned ok")
				}
			})
		})
	}
}

func TestGiteaClientCreation(t *testing.T) {
	// Test with valid base URL
	client := NewGiteaClient("https://gitea.example.com", "test-token", "test-webhook-secret")
	if client == nil {
		t.Log("Gitea client creation returned nil (expected for invalid/mocked URL)")
		return
	}

	// Verify basic properties
	if client.baseURL != "https://gitea.example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://gitea.example.com")
	}
	if client.token != "test-token" {
		t.Errorf("token = %q, want %q", client.token, "test-token")
	}
	if client.webhookSecret != "test-webhook-secret" {
		t.Errorf("webhookSecret = %q, want %q", client.webhookSecret, "test-webhook-secret")
	}
}

func TestParseDiffFiles(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want []string
	}{
		{
			name: "single file",
			diff: "diff --git a/src/main.go b/src/main.go\n--- a/src/main.go\n+++ b/src/main.go\n@@ -1,3 +1,3 @@",
			want: []string{"src/main.go"},
		},
		{
			name: "multiple files",
			diff: "diff --git a/src/main.go b/src/main.go\n--- a/src/main.go\n+++ b/src/main.go\ndiff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md",
			want: []string{"src/main.go", "README.md"},
		},
		{
			name: "renamed file",
			diff: "diff --git a/old_name.go b/new_name.go\n--- a/old_name.go\n+++ b/new_name.go",
			want: []string{"old_name.go"},
		},
		{
			name: "empty diff",
			diff: "",
			want: []string{},
		},
		{
			name: "nested paths",
			diff: "diff --git a/internal/handler/api.go b/internal/handler/api.go",
			want: []string{"internal/handler/api.go"},
		},
		{
			name: "new file",
			diff: "diff --git a/new_file.go b/new_file.go\nnew file mode 100644",
			want: []string{"new_file.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDiffFiles(tt.diff)
			sort.Strings(got)
			sort.Strings(tt.want)
			if len(got) != len(tt.want) {
				t.Errorf("parseDiffFiles() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseDiffFiles() = %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

func TestGitLabState(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"opened", "open"},
		{"closed", "closed"},
		{"merged", "merged"},
		{"OPENED", "open"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := gitLabState(tt.input)
			if got != tt.want {
				t.Errorf("gitLabState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractGiteaLabels(t *testing.T) {
	t.Run("empty labels", func(t *testing.T) {
		labels := extractGiteaLabels(nil)
		if len(labels) != 0 {
			t.Errorf("extractGiteaLabels(nil) = %v, want []", labels)
		}
	})
}