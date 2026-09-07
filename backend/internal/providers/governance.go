package providers

import "context"

// NotificationProvider is intentionally transport-neutral. Phase 2 uses an
// in-app implementation; email, chat and enterprise notification buses can be
// added without changing the control-plane domain.
type NotificationProvider interface {
	Notify(ctx context.Context, userID int64, notificationType, title, body string) error
}

type MockNotificationProvider struct{}

func (MockNotificationProvider) Notify(_ context.Context, _ int64, _, _, _ string) error { return nil }
