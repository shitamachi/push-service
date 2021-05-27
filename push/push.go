package push

import "context"

type Pusher interface {
	Push(ctx context.Context, appId, message, token string, data map[string]string) error
}
