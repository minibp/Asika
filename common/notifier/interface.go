package notifier

import "context"

// Notifier defines the interface for sending notifications
type Notifier interface {
	Type() string
	Send(ctx context.Context, title, body string) error
}
