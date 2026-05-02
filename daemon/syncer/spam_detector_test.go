package syncer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"asika/common/config"
	"asika/common/db"
	"asika/common/events"
	"asika/common/models"
	"asika/common/platforms"
	"asika/testutil"
)

func setupSpamTest(t *testing.T) (*SpamDetector, *testutil.MockPlatformClient, func()) {
	t.Helper()

	tdb := testutil.NewTestDB(t)
	db.DB = tdb

	cfg := &models.Config{
		Spam: models.SpamConfig{
			Enabled:          true,
			TimeWindow:       "10m",
			Threshold:        3,
			TriggerOnAuthor:  true,
			TriggerOnTitleKw: []string{"spam", "buy now", "click here"},
		},
		Notify: []models.NotifyConfig{},
		RepoGroups: []models.RepoGroupConfig{
			{
				Name:           "test-group",
				Mode:           "single",
				MirrorPlatform: "github",
				GitHub:         "owner/repo",
				DefaultBranch:  "main",
			},
		},
	}

	config.Store(cfg)
	events.Init()

	mock := testutil.NewMockPlatformClient()
	clients := map[platforms.PlatformType]platforms.PlatformClient{
		platforms.PlatformGitHub: mock,
	}

	sd := NewSpamDetectorWithClients(cfg, clients)

	cleanup := func() {
		db.Close()
	}

	return sd, mock, cleanup
}

func TestDetectSpamByAuthor(t *testing.T) {
	sd, _, cleanup := setupSpamTest(t)
	defer cleanup()

	now := time.Now()
	prs := []*models.PRRecord{
		{ID: "a", Author: "bot1", Title: "fix typo", PRNumber: 1, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "b", Author: "bot1", Title: "update docs", PRNumber: 2, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "c", Author: "bot1", Title: "minor change", PRNumber: 3, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "d", Author: "real-user", Title: "great feature", PRNumber: 4, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
	}

	spam := sd.detectSpam(prs)

	if len(spam) != 3 {
		t.Errorf("expected 3 spam PRs from bot1, got %d", len(spam))
	}

	for _, s := range spam {
		if s.Author == "real-user" {
			t.Errorf("real-user should not be marked as spam")
		}
	}
}

func TestDetectSpamByKeyword(t *testing.T) {
	sd, _, cleanup := setupSpamTest(t)
	defer cleanup()

	now := time.Now()
	prs := []*models.PRRecord{
		{ID: "a", Author: "user1", Title: "fix typo", PRNumber: 1, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "b", Author: "user2", Title: "Buy Now offer!!!", PRNumber: 2, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "c", Author: "user3", Title: "click here for deal", PRNumber: 3, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "d", Author: "user4", Title: "SPAM promotion", PRNumber: 4, CreatedAt: now, RepoGroup: "test-group", Platform: "github", State: "open"},
	}

	spam := sd.detectSpam(prs)

	if len(spam) != 3 {
		t.Errorf("expected 3 spam PRs by keyword, got %d", len(spam))
	}

	for _, s := range spam {
		if s.ID == "a" {
			t.Errorf("PR 'a' should not be spam by keyword")
		}
	}
}

func TestDetectSpamDisabled(t *testing.T) {
	sd, _, cleanup := setupSpamTest(t)
	defer cleanup()

	sd.cfg.Spam.Enabled = false

	prs := []*models.PRRecord{
		{ID: "a", Author: "bot1", Title: "spam", PRNumber: 1, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "b", Author: "bot1", Title: "also spam", PRNumber: 2, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "c", Author: "bot1", Title: "even more spam", PRNumber: 3, RepoGroup: "test-group", Platform: "github", State: "open"},
		{ID: "d", Author: "bot1", Title: "final spam", PRNumber: 4, RepoGroup: "test-group", Platform: "github", State: "open"},
	}

	spam := sd.detectSpam(prs)
	// Even thought disabled, detectSpam still works but Scan() checks cfg.Spam.Enabled first

	if len(spam) != 4 {
		t.Errorf("expected 4 spam PRs (detection ignores enabled flag, Scan checks it), got %d", len(spam))
	}
}

func TestHandleSpam(t *testing.T) {
	sd, mock, cleanup := setupSpamTest(t)
	defer cleanup()

	pr := &models.PRRecord{
		ID:        "test-id",
		RepoGroup: "test-group",
		Platform:  "github",
		PRNumber:  42,
		Title:     "Buy Now cheap!",
		Author:    "spammer",
		State:     "open",
	}

	sd.HandleSpam(pr, "test-group")

	if !pr.SpamFlag {
		t.Errorf("expected SpamFlag to be true")
	}

	if pr.State != "spam" {
		t.Errorf("expected State to be 'spam', got %q", pr.State)
	}

	ctx := context.Background()
	prs, _ := mock.ListPRs(ctx, "owner", "repo", "")

	_ = prs

	key := "test-group#github#42"
	data, err := db.Get(db.BucketPRs, key)
	if err != nil {
		t.Fatalf("failed to get PR from DB: %v", err)
	}

	var stored models.PRRecord
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("failed to unmarshal stored PR: %v", err)
	}

	if !stored.SpamFlag {
		t.Errorf("stored PR should have SpamFlag=true")
	}

	if stored.State != "spam" {
		t.Errorf("stored PR should have state='spam', got %q", stored.State)
	}
}
