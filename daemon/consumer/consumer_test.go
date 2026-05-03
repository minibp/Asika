package consumer

import (
	"testing"

	"asika/common/events"
	"asika/common/models"
	"asika/common/platforms"
)

func TestNewConsumer(t *testing.T) {
	c := NewConsumer()
	if c == nil {
		t.Fatal("NewConsumer returned nil")
	}
}

func TestNewConsumerWithClients(t *testing.T) {
	cfg := &models.Config{}
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	c := NewConsumerWithClients(cfg, clients)
	if c == nil {
		t.Fatal("NewConsumerWithClients returned nil")
	}
}

func TestStartStop(t *testing.T) {
	events.Init()

	c := NewConsumer()

	// Start consumer in a goroutine
	go c.Start()

	// Slightly wait
	// time.Sleep(10 * time.Millisecond)

	// Stop consumer
	c.Stop()

	// Test passes if no panic
}

func TestSetStaleManager(t *testing.T) {
	c := NewConsumer()

	// Create a fake stale.Manager (we only test that the function doesn't panic)
	// Since stale.Manager is a struct, we can create it directly
	c.SetStaleManager(nil) // Pass nil, should be fine

	// Test passes if no panic
}

func TestHandleEvent_NilPR(t *testing.T) {
	c := NewConsumer()

	// Send an event with nil PR
	event := events.Event{
		Type:      events.EventPROpened,
		RepoGroup: "main",
		Platform:  "github",
		PR:        nil,
	}

	// Should not panic
	c.handleEvent(event)
}

func TestHandleEvent_InvalidType(t *testing.T) {
	c := NewConsumer()

	event := events.Event{
		Type:      "invalid_type",
		RepoGroup: "main",
		Platform:  "github",
		PR:        nil,
	}

	// Should not panic (just won't handle)
	c.handleEvent(event)
}
