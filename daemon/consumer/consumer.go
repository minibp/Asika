package consumer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"asika/common/db"
	"asika/common/events"
)

// Consumer consumes events and processes them
type Consumer struct {
	stop chan struct{}
}

// NewConsumer creates a new event consumer
func NewConsumer() *Consumer {
	return &Consumer{
		stop: make(chan struct{}),
	}
}

// Start starts consuming events
func (c *Consumer) Start() {
	ch := events.Subscribe()
	go func() {
		for {
			select {
			case event := <-ch:
				c.handleEvent(event)
			case <-c.stop:
				slog.Info("event consumer stopped")
				return
			}
		}
	}()
}

// Stop stops the consumer
func (c *Consumer) Stop() {
	close(c.stop)
}

func (c *Consumer) handleEvent(event events.Event) {
	slog.Info("received event", "type", event.Type, "repo_group", event.RepoGroup, "platform", event.Platform)

	switch event.Type {
	case events.EventPROpened:
		c.handlePROpened(event)
	case events.EventPRClosed:
		c.handlePRClosed(event)
	case events.EventPRMerged:
		c.handlePRMerged(event)
	case events.EventPRApproved:
		c.handlePRApproved(event)
	case events.EventSpamDetected:
		c.handleSpamDetected(event)
	}
}

func (c *Consumer) handlePROpened(event events.Event) {
	pr := event.PR
	if pr == nil {
		return
	}

	slog.Info("PR opened", "title", pr.Title, "author", pr.Author)

	// TODO:
	// 1. Store in bbolt (bucket: prs)
	// 2. Trigger label rule engine
	// 3. Record PREvent to PR's Events list
}

func (c *Consumer) handlePRClosed(event events.Event) {
	pr := event.PR
	if pr == nil {
		return
	}

	slog.Info("PR closed", "title", pr.Title)

	// Update state in bbolt
	pr.State = "closed"
	pr.UpdatedAt = time.Now()
	key := fmt.Sprintf("%s#%s#%d", event.RepoGroup, event.Platform, pr.PRNumber)
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, key, data)
}

func (c *Consumer) handlePRMerged(event events.Event) {
	pr := event.PR
	if pr == nil {
		return
	}

	slog.Info("PR merged", "title", pr.Title)

	// Update state in bbolt
	pr.State = "merged"
	pr.UpdatedAt = time.Now()
	key := fmt.Sprintf("%s#%s#%d", event.RepoGroup, event.Platform, pr.PRNumber)
	data, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, key, data)

	// TODO: trigger code sync (multi mode only)
}

func (c *Consumer) handlePRApproved(event events.Event) {
	pr := event.PR
	if pr == nil {
		return
	}

	slog.Info("PR approved", "title", pr.Title)

	// Check if PR should be added to merge queue
	// TODO: implement queue.AddToQueue logic
	// queue.AddToQueue(pr, event.RepoGroup)
}

func (c *Consumer) handleSpamDetected(event events.Event) {
	pr := event.PR
	if pr == nil {
		return
	}

	slog.Warn("spam detected", "title", pr.Title, "author", pr.Author)

	// TODO: handle spam (close PR, notify admin)
	// 1. Update PR state to spam
	// 2. Call platform client to close PR
	// 3. Send notifications
}
