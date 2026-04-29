package notifier

import (
    "context"
    "fmt"
    "log/slog"
    "strings"

    "github.com/wneessen/go-mail"
)

// SMTPNotifier sends notifications via SMTP
type SMTPNotifier struct {
    host     string
    port     int
    username string
    password string
    to       []string
}

// NewSMTPNotifier creates a new SMTP notifier
func NewSMTPNotifier(config map[string]interface{}) *SMTPNotifier {
    host, _ := config["host"].(string)
    portFloat, _ := config["port"].(float64)
    port := int(portFloat)
    username, _ := config["username"].(string)
    password, _ := config["password"].(string)

    to := make([]string, 0)
    if toList, ok := config["to"].([]interface{}); ok {
        for _, t := range toList {
            if s, ok := t.(string); ok {
                to = append(to, s)
            }
        }
    }

    return &SMTPNotifier{
        host:     host,
        port:     port,
        username: username,
        password: password,
        to:       to,
    }
}

// Type returns the type of notifier
func (n *SMTPNotifier) Type() string {
    return "smtp"
}

// Send sends a notification
func (n *SMTPNotifier) Send(ctx context.Context, title, body string) error {
    if len(n.to) == 0 {
        return fmt.Errorf("no recipients configured")
    }

    client, err := mail.NewClient(n.host, mail.WithPort(n.port), mail.WithSMTPAuth(mail.SMTPAuthPlain), mail.WithUsername(n.username), mail.WithPassword(n.password))
    if err != nil {
        return fmt.Errorf("failed to create mail client: %w", err)
    }
    defer client.Close()

    msg := mail.NewMsg()
    if err := msg.From(n.username); err != nil {
        return err
    }

    toAddresses := make([]string, len(n.to))
    copy(toAddresses, n.to)
    if err := msg.To(toAddresses...); err != nil {
        return err
    }

    msg.Subject(title)
    msg.SetBodyString("text/plain", body)

    if err := client.DialAndSend(msg); err != nil {
        slog.Error("failed to send email", "error", err)
        return err
    }

    slog.Info("email sent", "to", strings.Join(n.to, ","))
    return nil
}
