package feishu

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

func setupFeishuTest(t *testing.T) (*Bot, func()) {
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
		Feishu: models.FeishuConfig{
			Enabled:   true,
			AppID:     "test-app-id",
			AppSecret: "test-secret",
			AdminIDs:  []string{"ou_admin1", "ou_admin2"},
		},
	}
	config.Store(cfg)

	qm := queue.NewManager(cfg, clients)
	syncr := syncer.NewSyncer(cfg, clients)
	sd := syncer.NewSpamDetectorWithClients(cfg, clients)

	b := NewBot(cfg, clients, qm, syncr, sd, nil)

	cleanup := func() {
		db.Close()
	}
	return b, cleanup
}

func TestFeishuBotCreation(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	if b == nil {
		t.Fatal("bot should not be nil")
	}
	if len(b.adminIDs) != 2 {
		t.Errorf("expected 2 admin IDs, got %d", len(b.adminIDs))
	}
	if !b.adminIDs["ou_admin1"] {
		t.Error("expected admin ID ou_admin1 to be present")
	}
	if !b.adminIDs["ou_admin2"] {
		t.Error("expected admin ID ou_admin2 to be present")
	}
}

func TestFeishuIsAdmin_WithAdminIDs(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	if !b.isAdmin("ou_admin1") {
		t.Error("admin ou_admin1 should be recognized")
	}

	if b.isAdmin("ou_stranger") {
		t.Error("stranger should not be admin")
	}
}

func TestFeishuIsAdmin_EmptyAdminIDs(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()
	b.adminIDs = map[string]bool{}

	if !b.isAdmin("anyone") {
		t.Error("with empty adminIDs, anyone should be admin")
	}
}

func TestParseMessageText(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"JSON text message", `{"text":"help"}`, "help"},
		{"JSON with spaces", `{"text":"prs mygroup"}`, "prs mygroup"},
		{"Plain text", "help", "help"},
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.parseMessageText(tt.content)
			if got != tt.want {
				t.Errorf("parseMessageText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcessCommand_Help(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "help")
	if reply == "" {
		t.Error("help should return text")
	}
	if !containsStr(reply, "help") && !containsStr(reply, "Help") {
		t.Error("help response should contain help info")
	}
}

func TestProcessCommand_Unknown(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "xyzzy")
	if reply == "" {
		t.Error("unknown command should return a message")
	}
}

func TestProcessCommand_AccessDenied(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_stranger", "help")
	if reply == "" || !containsStr(reply, "Access") {
		t.Error("stranger should get access denied")
	}
}

func TestProcessCommand_PRs(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "prs")
	if reply == "" {
		t.Error("prs should return text")
	}
}

func TestProcessCommand_PrWrongArgs(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "pr group")
	if !containsStr(reply, "Usage") {
		t.Errorf("wrong pr args should show usage, got: %s", reply)
	}
}

func TestProcessCommand_Config(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "config")
	if reply == "" || !containsStr(reply, "Config") {
		t.Error("config should return config info")
	}
}

func TestProcessCommand_QueueEmpty(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "queue")
	if reply == "" {
		t.Error("queue should return text")
	}
}

func TestProcessCommand_Recheck(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	reply := b.processCommand("ou_admin1", "recheck")
	if reply == "" {
		t.Error("recheck should return text")
	}
}

func TestFeishuBotStartStop(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	// Start and stop should not panic
	b.Start()
	b.Stop()
}

func TestFeishuURLVerification(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	challenge := `{"challenge":"test123","token":"abc","type":"url_verification"}`
	var raw json.RawMessage
	json.Unmarshal([]byte(challenge), &raw)

	resp, err := b.handleURLVerification(raw)
	if err != nil {
		t.Fatalf("handleURLVerification failed: %v", err)
	}

	respMap, _ := resp.(map[string]string)
	if respMap["challenge"] != "test123" {
		t.Errorf("challenge = %q, want test123", respMap["challenge"])
	}
}

func TestHandleEvent_URLVerification(t *testing.T) {
	b, cleanup := setupFeishuTest(t)
	defer cleanup()

	body := []byte(`{"schema":"2.0","header":{"event_type":"url_verification","token":"x"},"event":{"challenge":"chal456","token":"abc","type":"url_verification"}}`)

	resp, err := b.HandleEvent(nil, body)
	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}

	respMap, ok := resp.(map[string]string)
	if !ok || respMap["challenge"] != "chal456" {
		t.Errorf("expected challenge=chal456, got %v", resp)
	}
}

func TestFeishuMarkSpamLogic(t *testing.T) {
	_, cleanup := setupFeishuTest(t)
	defer cleanup()

	pr := models.PRRecord{
		ID:        "fs-spam-pr",
		RepoGroup: "test-group",
		Platform:  "github",
		PRNumber:  77,
		Title:     "Feishu spam test",
		Author:    "bot77",
		State:     "open",
	}
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, "test-group#github#77", data)

	found, _ := getPRRecord("test-group", "77")
	if found == nil {
		t.Fatal("PR should exist")
	}
	if found.State != "open" {
		t.Errorf("initial state = %q, want open", found.State)
	}
}

func TestGetPRRecord_NotFound(t *testing.T) {
	_, cleanup := setupFeishuTest(t)
	defer cleanup()

	_, err := getPRRecord("test-group", "99999")
	if err == nil {
		t.Error("expected error for non-existent PR")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}