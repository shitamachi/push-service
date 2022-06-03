package push

import (
	"context"
	"github.com/shitamachi/push-service/models"
)

type Pusher interface {
	GetClientByAppID(ctx context.Context, appId string) (interface{}, bool)
	Push(ctx context.Context, message *models.PushMessage) (interface{}, error)
}
