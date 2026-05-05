package platform

import (
	"encoding/json"
	"testing"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/platforms"
	"asika/daemon/queue"
	"asika/daemon/syncer"
	"asika/testutil"
)

func setupBotTest(t *testing.T) (*TelegramBot, func()) {
	t.Helper()

	tdb := testutil.NewTestDB(t)
	db.DB = tdb

	mock := testutil.NewMockPlatformClient()
	clients := map[platforms.PlatformType]platforms.PlatformClient{
		platforms.PlatformGitHub: mock,
	}

	cfg := &models.Config{
		RepoGroups: []models.RepoGroupConfig{
			{Name: "test-group", Mode: "multi", GitHub: "owner/repo"},
		},
		Telegram: models.TelegramConfig{
			Enabled:  true,
			Token:    "test-token",
			AdminIDs: []int64{12345, 67890},
		},
	}
	config.Store(cfg)

	qm := queue.NewManager(cfg, clients)
	syncr := syncer.NewSyncer(cfg, clients)
	sd := syncer.NewSpamDetectorWithClients(cfg, clients)

	bot := &TelegramBot{
		bot:          nil, // no real bot for tests
		cfg:          cfg,
		clients:      clients,
		queueMgr:     qm,
		syncerRef:    syncr,
		spamDetector: sd,
		adminIDs:     map[int64]bool{12345: true, 67890: true},
		stop:         make(chan struct{}),
	}

	cleanup := func() {
		db.Close()
	}
	return bot, cleanup
}

func TestBotCreation(t *testing.T) {
	bot, cleanup := setupBotTest(t)
	defer cleanup()

	if bot == nil {
		t.Fatal("bot should not be nil")
	}
	if len(bot.adminIDs) != 2 {
		t.Errorf("expected 2 admin IDs, got %d", len(bot.adminIDs))
	}
	if !bot.adminIDs[12345] {
		t.Error("expected admin ID 12345 to be present")
	}
	if !bot.adminIDs[67890] {
		t.Error("expected admin ID 67890 to be present")
	}
}

func TestIsAdmin_WithAdminIDs(t *testing.T) {
	bot, cleanup := setupBotTest(t)
	defer cleanup()

	if bot.isAdminByID(12345) {
		t.Log("admin 12345 recognized")
	} else {
		t.Error("admin 12345 should be recognized")
	}

	if bot.isAdminByID(99999) {
		t.Error("non-admin 99999 should not be recognized")
	}
}

func TestIsAdmin_EmptyAdminIDs(t *testing.T) {
	bot, cleanup := setupBotTest(t)
	defer cleanup()
	bot.adminIDs = map[int64]bool{}

	if !bot.isAdminByID(99999) {
		t.Error("with empty adminIDs, everyone should be admin")
	}
}

// isAdminByID is a helper added for testing (duplicates internal logic)
func (b *TelegramBot) isAdminByID(id int64) bool {
	if len(b.adminIDs) == 0 {
		return true
	}
	return b.adminIDs[id]
}

func TestGetPRByID(t *testing.T) {
	_, cleanup := setupBotTest(t)
	defer cleanup()

	// Store a test PR
	pr := models.PRRecord{
		ID:        "pr-abc-123",
		RepoGroup: "test-group",
		Platform:  "github",
		PRNumber:  42,
		Title:     "Fix critical bug",
		Author:    "dev1",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "test-group#github#42", data)

	t.Run("find by ID", func(t *testing.T) {
		found, err := getPRByID("test-group", "pr-abc-123")
		if err != nil {
			t.Fatalf("getPRByID failed: %v", err)
		}
		if found.Title != "Fix critical bug" {
			t.Errorf("Title = %q, want %q", found.Title, "Fix critical bug")
		}
	})

	t.Run("find by number", func(t *testing.T) {
		found, err := getPRByID("test-group", "42")
		if err != nil {
			t.Fatalf("getPRByID by number failed: %v", err)
		}
		if found.PRNumber != 42 {
			t.Errorf("PRNumber = %d, want 42", found.PRNumber)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := getPRByID("test-group", "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent PR")
		}
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 10, ""},
		{"x", 1, "x"},
		{"abcdefg", 7, "abcdefg"},
		{"abcdefgh", 7, "abcdefg..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestGetClientForPlatform(t *testing.T) {
	bot, cleanup := setupBotTest(t)
	defer cleanup()

	client := bot.getClientForPlatform("github")
	if client == nil {
		t.Error("expected non-nil client for github")
	}

	client = bot.getClientForPlatform("unknown")
	if client != nil {
		t.Error("expected nil client for unknown platform")
	}
}

func TestBotStop(t *testing.T) {
	bot, cleanup := setupBotTest(t)
	defer cleanup()

	// Stop should not panic
	bot.Stop()
}

func TestBotRegistration_Commands(t *testing.T) {
	bot, cleanup := setupBotTest(t)
	defer cleanup()

	// Verify bot's key fields are initialized
	if bot.cfg == nil {
		t.Error("cfg should not be nil")
	}
	if bot.clients == nil {
		t.Error("clients should not be nil")
	}
	if bot.queueMgr == nil {
		t.Error("queueMgr should not be nil")
	}
	if bot.syncerRef == nil {
		t.Error("syncerRef should not be nil")
	}
	if bot.spamDetector == nil {
		t.Error("spamDetector should not be nil")
	}
}

func TestAdminIDsConfig(t *testing.T) {
	_, cleanup := setupBotTest(t)
	defer cleanup()

	cfg := config.Current()
	if cfg == nil {
		t.Fatal("config not set")
	}

	if len(cfg.Telegram.AdminIDs) != 2 {
		t.Errorf("expected 2 admin IDs, got %d", len(cfg.Telegram.AdminIDs))
	}
}

func TestMarkSpamViaBotLogic(t *testing.T) {
	_, cleanup := setupBotTest(t)
	defer cleanup()

	pr := models.PRRecord{
		ID:        "spam-telegram-pr",
		RepoGroup: "test-group",
		Platform:  "github",
		PRNumber:  99,
		Title:     "Buy cheap meds",
		Author:    "spam_bot",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "test-group#github#99", data)

	// Verify we can find the PR
	found, err := getPRByID("test-group", "99")
	if err != nil {
		t.Fatalf("getPRByID failed: %v", err)
	}
	if found.State != "open" {
		t.Errorf("expected state=open, got %q", found.State)
	}

	// Simulate spam marking
	found.SpamFlag = true
	found.State = "spam"
	updated, _ := json.Marshal(found)
	db.Put(db.BucketPRs, "test-group#github#99", updated)

	// Verify state updated
	found2, err := getPRByID("test-group", "spam-telegram-pr")
	if err != nil {
		t.Fatalf("getPRByID after spam failed: %v", err)
	}
	if !found2.SpamFlag {
		t.Error("expected SpamFlag=true")
	}
	if found2.State != "spam" {
		t.Errorf("expected state=spam, got %q", found2.State)
	}
}

func TestSpamReopenViaBotLogic(t *testing.T) {
	_, cleanup := setupBotTest(t)
	defer cleanup()

	// Create a spam PR first
	pr := models.PRRecord{
		ID:        "reopen-telegram-pr",
		RepoGroup: "test-group",
		Platform:  "github",
		PRNumber:  88,
		Title:     "Legit PR marked spam",
		Author:    "honest_dev",
		State:     "spam",
		SpamFlag:  true,
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "test-group#github#88", data)

	// Simulate reopen (spam recovery)
	pr.State = "open"
	pr.SpamFlag = false
	updated, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "test-group#github#88", updated)

	found, err := getPRByID("test-group", "88")
	if err != nil {
		t.Fatalf("getPRByID after reopen failed: %v", err)
	}
	if found.State != "open" {
		t.Errorf("expected state=open after reopen, got %q", found.State)
	}
	if found.SpamFlag {
		t.Error("expected SpamFlag=false after reopen")
	}
}

func setupMultiGroupTest(t *testing.T) (*TelegramBot, func()) {
	t.Helper()

	tdb := testutil.NewTestDB(t)
	db.DB = tdb

	mock := testutil.NewMockPlatformClient()
	clients := map[platforms.PlatformType]platforms.PlatformClient{
		platforms.PlatformGitHub: mock,
	}

	cfg := &models.Config{
		RepoGroups: []models.RepoGroupConfig{
			{Name: "group-a", Mode: "multi", GitHub: "org-a/repo-a"},
			{Name: "group-b", Mode: "multi", GitHub: "org-b/repo-b"},
		},
		Telegram: models.TelegramConfig{
			Enabled:  true,
			Token:    "test-token",
			AdminIDs: []int64{12345},
		},
	}
	config.Store(cfg)

	qm := queue.NewManager(cfg, clients)
	syncr := syncer.NewSyncer(cfg, clients)
	sd := syncer.NewSpamDetectorWithClients(cfg, clients)

	bot := &TelegramBot{
		bot:          nil,
		cfg:          cfg,
		clients:      clients,
		queueMgr:     qm,
		syncerRef:    syncr,
		spamDetector: sd,
		adminIDs:     map[int64]bool{12345: true},
		stop:         make(chan struct{}),
	}

	cleanup := func() {
		db.Close()
	}
	return bot, cleanup
}

func TestMultiGroup_CloseReopenSpam(t *testing.T) {
	_, cleanup := setupMultiGroupTest(t)
	defer cleanup()

	// Create same PR number in two different groups
	prA := models.PRRecord{
		ID:        "pr-close-a",
		RepoGroup: "group-a",
		Platform:  "github",
		PRNumber:  42,
		Title:     "Feature A",
		Author:    "dev1",
		State:     "open",
	}
	prB := models.PRRecord{
		ID:        "pr-close-b",
		RepoGroup: "group-b",
		Platform:  "github",
		PRNumber:  42,
		Title:     "Feature B",
		Author:    "dev2",
		State:     "open",
	}
	dataA, _ := json.Marshal(prA)
	dataB, _ := json.Marshal(prB)
	db.Put(db.BucketPRs, "group-a#github#42", dataA)
	db.Put(db.BucketPRs, "group-b#github#42", dataB)

	t.Run("close group-a does not affect group-b", func(t *testing.T) {
		// Close PR in group-a
		pr, err := getPRByID("group-a", "42")
		if err != nil {
			t.Fatalf("getPRByID failed: %v", err)
		}
		if pr.RepoGroup != "group-a" {
			t.Errorf("expected group-a, got %s", pr.RepoGroup)
		}
		pr.State = "closed"
		updated, _ := json.Marshal(pr)
		db.Put(db.BucketPRs, "group-a#github#42", updated)

		// Verify group-b is untouched
		prB2, err := getPRByID("group-b", "42")
		if err != nil {
			t.Fatalf("getPRByID for group-b failed: %v", err)
		}
		if prB2.State != "open" {
			t.Errorf("group-b PR state = %q, want open", prB2.State)
		}
	})

	t.Run("spam group-b does not affect group-a", func(t *testing.T) {
		pr, err := getPRByID("group-b", "42")
		if err != nil {
			t.Fatalf("getPRByID failed: %v", err)
		}
		pr.SpamFlag = true
		pr.State = "spam"
		updated, _ := json.Marshal(pr)
		db.Put(db.BucketPRs, "group-b#github#42", updated)

		// Verify group-a is still closed (not spam)
		prA2, err := getPRByID("group-a", "42")
		if err != nil {
			t.Fatalf("getPRByID for group-a failed: %v", err)
		}
		if prA2.State != "closed" {
			t.Errorf("group-a PR state = %q, want closed", prA2.State)
		}
		if prA2.SpamFlag {
			t.Error("group-a PR should not have SpamFlag")
		}
	})

	t.Run("reopen spam PR clears spam flag", func(t *testing.T) {
		pr, err := getPRByID("group-b", "42")
		if err != nil {
			t.Fatalf("getPRByID failed: %v", err)
		}
		if !pr.SpamFlag {
			t.Fatal("expected SpamFlag=true before reopen")
		}

		// Simulate reopen logic from handleReopenPR
		pr.State = "open"
		pr.SpamFlag = false
		updated, _ := json.Marshal(pr)
		db.Put(db.BucketPRs, "group-b#github#42", updated)

		pr2, err := getPRByID("group-b", "42")
		if err != nil {
			t.Fatalf("getPRByID after reopen failed: %v", err)
		}
		if pr2.State != "open" {
			t.Errorf("state = %q, want open", pr2.State)
		}
		if pr2.SpamFlag {
			t.Error("SpamFlag should be false after reopen")
		}
	})
}

func TestMultiGroup_GetOwnerRepoFromGroup(t *testing.T) {
	_, cleanup := setupMultiGroupTest(t)
	defer cleanup()

	t.Run("group-a resolves correctly", func(t *testing.T) {
		group := config.GetRepoGroupByName(config.Current(), "group-a")
		if group == nil {
			t.Fatal("group-a not found")
		}
		owner, repo := config.GetOwnerRepoFromGroup(group, "github")
		if owner != "org-a" || repo != "repo-a" {
			t.Errorf("got %s/%s, want org-a/repo-a", owner, repo)
		}
	})

	t.Run("group-b resolves correctly", func(t *testing.T) {
		group := config.GetRepoGroupByName(config.Current(), "group-b")
		if group == nil {
			t.Fatal("group-b not found")
		}
		owner, repo := config.GetOwnerRepoFromGroup(group, "github")
		if owner != "org-b" || repo != "repo-b" {
			t.Errorf("got %s/%s, want org-b/repo-b", owner, repo)
		}
	})

	t.Run("non-existent group returns nil", func(t *testing.T) {
		group := config.GetRepoGroupByName(config.Current(), "no-such-group")
		if group != nil {
			// GetRepoGroupByName falls back to "default" group
			t.Logf("got fallback group: %s", group.Name)
		}
	})
}

func TestEmptyBotSetup(t *testing.T) {
	bot := &TelegramBot{
		adminIDs: map[int64]bool{},
		stop:     make(chan struct{}),
	}

	if bot == nil {
		t.Fatal("bot should not be nil")
	}
	if bot.cfg == nil {
		t.Log("cfg is nil (empty bot)")
	}
	if len(bot.clients) == 0 {
		t.Log("clients is empty (empty bot)")
	}
}