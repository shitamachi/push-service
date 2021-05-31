package push

import "context"

type Pusher interface {
	GetClientByAppID(appId string) (interface{}, bool)
	Push(ctx context.Context, appId, message, token string, data map[string]string) (interface{}, error)
	//GetClientAndPush(ctx context.Context, appId, message, token string, data map[string]string) error
}
