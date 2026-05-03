package queue

import (
	"encoding/json"
	"testing"

	"asika/common/db"
	"asika/common/models"
	"asika/common/platforms"
	"asika/testutil"
)

func TestNewManager(t *testing.T) {
	cfg := &models.Config{}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	m := NewManager(cfg, clients)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewChecker(t *testing.T) {
	cfg := &models.Config{}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	c := NewChecker(cfg, clients)
	if c == nil {
		t.Fatal("NewChecker returned nil")
	}
}

func TestAddToQueue(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	cfg := &models.Config{
		RepoGroups: []models.RepoGroupConfig{
			{
				Name: "main",
				Mode: "multi",
			},
		},
	}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	m := NewManager(cfg, clients)

	pr := &models.PRRecord{
		ID:        "pr-123",
		RepoGroup: "main",
		Platform:  "github",
		PRNumber:  1,
		Title:     "Test PR",
		State:     "open",
	}

	err := m.AddToQueue(pr)
	if err != nil {
		t.Fatalf("AddToQueue failed: %v", err)
	}
}

func TestAddToQueue_Duplicate(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	cfg := &models.Config{}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	m := NewManager(cfg, clients)

	pr := &models.PRRecord{
		ID:       "pr-123",
		RepoGroup: "main",
		Platform:  "github",
		PRNumber:  1,
		Title:     "Test PR",
		State:     "open",
	}

	// First add
	err := m.AddToQueue(pr)
	if err != nil {
		t.Fatalf("First AddToQueue failed: %v", err)
	}

	// Second add (should not error)
	err = m.AddToQueue(pr)
	if err != nil {
		t.Fatalf("Second AddToQueue should not error: %v", err)
	}
}

func TestGetQueueItems(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	cfg := &models.Config{}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	m := NewManager(cfg, clients)

	// Add several PRs to queue
	prs := []*models.PRRecord{
		{ID: "pr-1", RepoGroup: "main", Platform: "github", PRNumber: 1, Title: "PR 1", State: "open"},
		{ID: "pr-2", RepoGroup: "main", Platform: "github", PRNumber: 2, Title: "PR 2", State: "open"},
		{ID: "pr-3", RepoGroup: "other", Platform: "gitlab", PRNumber: 3, Title: "PR 3", State: "open"},
	}

	for _, pr := range prs {
		m.AddToQueue(pr)
	}

	// Get queue items for main repo group
	items, err := m.GetQueueItems("main")
	if err != nil {
		t.Fatalf("GetQueueItems failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("GetQueueItems returned %d items, want 2", len(items))
	}
}

func TestGetQueueItems_Empty(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	cfg := &models.Config{}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	m := NewManager(cfg, clients)

	items, err := m.GetQueueItems("main")
	if err != nil {
		t.Fatalf("GetQueueItems failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("GetQueueItems returned %d items, want 0", len(items))
	}
}

func TestShouldMerge(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	cfg := &models.Config{
		RepoGroups: []models.RepoGroupConfig{
			{
				Name:   "main",
				GitHub: "owner/repo",
				MergeQueue: models.MergeQueueConfig{
					RequiredApprovals: 1,
					CICheckRequired:   false,
					CoreContributors:  []string{"user1"},
				},
			},
		},
	}

	client := &testutil.MockPlatformClient{
		Approvals: []string{"user1"},
		CIStatus:  "success",
	}

	clients := map[platforms.PlatformType]platforms.PlatformClient{
		platforms.PlatformGitHub: client,
	}

	c := NewChecker(cfg, clients)

	// Add PR to database
	pr := &models.PRRecord{
		ID:        "pr-123",
		RepoGroup: "main",
		Platform:  "github",
		PRNumber:  1,
		Title:     "Test PR",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "main#github#1", data)

	item := &models.QueueItem{
		PRID:     "pr-123",
		RepoGroup: "main",
		Status:    "waiting",
	}

	shouldMerge, err := c.ShouldMerge(item)
	if err != nil {
		t.Fatalf("ShouldMerge failed: %v", err)
	}

	if !shouldMerge {
		t.Error("ShouldMerge should return true for pr with enough approvals")
	}
}

func TestShouldMerge_NotEnoughApprovals(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	cfg := &models.Config{
		RepoGroups: []models.RepoGroupConfig{
			{
				Name:   "main",
				GitHub: "owner/repo",
				MergeQueue: models.MergeQueueConfig{
					RequiredApprovals: 2,
				},
			},
		},
	}

	client := &testutil.MockPlatformClient{
		Approvals: []string{"user1"}, // Only 1 approval
	}

	clients := map[platforms.PlatformType]platforms.PlatformClient{
		platforms.PlatformGitHub: client,
	}

	c := NewChecker(cfg, clients)

	// Add PR to database
	pr := &models.PRRecord{
		ID:        "pr-123",
		RepoGroup: "main",
		Platform:  "github",
		PRNumber:  1,
		Title:     "Test PR",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "main#github#1", data)

	item := &models.QueueItem{
		PRID:     "pr-123",
		RepoGroup: "main",
		Status:    "waiting",
	}

	shouldMerge, err := c.ShouldMerge(item)
	if err != nil {
		t.Fatalf("ShouldMerge failed: %v", err)
	}

	if shouldMerge {
		t.Error("ShouldMerge should return false when not enough approvals")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		list  []string
		item  string
		want  bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.item, func(t *testing.T) {
			got := contains(tt.list, tt.item)
			if got != tt.want {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.list, tt.item, got, tt.want)
			}
		})
	}
}

func TestFindPRByID(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	// Add PR to database
	pr := &models.PRRecord{
		ID:        "test-pr-id",
		RepoGroup: "main",
		Platform:  "github",
		PRNumber:  1,
		Title:     "Test PR",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "main#github#1", data)

	// Find PR
	found, err := findPRByID("test-pr-id")
	if err != nil {
		t.Fatalf("findPRByID failed: %v", err)
	}

	if found == nil {
		t.Fatal("findPRByID returned nil")
	}
	if found.ID != "test-pr-id" {
		t.Errorf("found.ID = %q, want test-pr-id", found.ID)
	}
}

func TestGetPRFromDB(t *testing.T) {
	dir := t.TempDir()
	db.Init(dir + "/test.db")
	t.Cleanup(func() { db.Close() })

	// Add PR to database
	pr := &models.PRRecord{
		ID:        "test-pr-id",
		RepoGroup: "main",
		Platform:  "github",
		PRNumber:  1,
		Title:     "Test PR",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "main#test-pr-id", data)

	// Get PR
	found, err := getPRFromDB("main", "test-pr-id")
	if err != nil {
		t.Fatalf("getPRFromDB failed: %v", err)
	}

	if found == nil {
		t.Fatal("getPRFromDB returned nil")
	}
	if found.ID != "test-pr-id" {
		t.Errorf("found.ID = %q, want test-pr-id", found.ID)
	}
}
