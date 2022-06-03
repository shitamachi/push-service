package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/mitchellh/mapstructure"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/models"
	"github.com/shitamachi/push-service/push"
	"github.com/shitamachi/redisqueue/v2"
	"go.uber.org/zap"
	"time"
)

type PushStreamMessage struct {
	models.BaseMessage `mapstructure:",squash"`
	AppId              string `json:"app_id" mapstructure:"app_id"`
	Token              string `json:"token" mapstructure:"token"`
	UserId             string `json:"user_id" mapstructure:"user_id"`
	ActionId           string `json:"action_id" mapstructure:"action_id"`
}

func ProcessPushMessage(ctx context.Context, message *redisqueue.Message) (err error) {
	var psm = new(PushStreamMessage)
	psm.Data = psm.BaseMessage.DecodeData(ctx)
	err = mapstructure.Decode(message.Values, psm)
	if err != nil {
		log.WithCtx(ctx).Error("Push: can not decode map to struct", zap.Any("message", message))
		return fmt.Errorf("can not decode map to struct PushStreamMessage")
	}

	client, err := getPushClientByAppId(ctx, psm.AppId)
	if err != nil {
		log.WithCtx(ctx).Warn("Push: can not get push message client by app id", zap.String("app_id", psm.AppId))
		return
	}

	err = backoff.RetryNotify(
		// operation func
		func() error {
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			_, err = client.Push(ctx, models.NewPushMessage(psm.AppId, psm.Token).SetBaseMessage(psm.BaseMessage))
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				log.WithCtx(ctx).Warn("Push: send push message timeout")
				return err
			case err != nil:
				// should not retry
				log.WithCtx(ctx).Info("Push: push notification failed",
					zap.Error(err),
					zap.String("app_id", psm.AppId),
					zap.Any("message", psm.BaseMessage),
				)
				return backoff.Permanent(err)
			}
			return err
		},
		// backoff policy
		backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3),
		// notify func
		func(err error, duration time.Duration) {
			log.WithCtx(ctx).Info("Push: failed to push message, will retry push again",
				zap.String("app_id", psm.AppId),
				zap.Error(err),
			)
		})

	if err != nil {
		log.WithCtx(ctx).Error("Push: failed to push message",
			zap.String("app_id", psm.AppId),
			zap.Error(err),
			zap.Any("message", psm.BaseMessage),
		)
	}

	return
}

func getPushClientByAppId(ctx context.Context, appID string) (push.Pusher, error) {
	conf := config.GetFromContext(ctx)
	err := fmt.Errorf("can not get push client item form config by app id=\"%s\"", appID)

	if len(appID) <= 0 {
		return nil, err
	}

	item, ok := conf.ClientConfig[appID]
	if ok {
		switch item.PushType {
		case config_entries.ApplePush:
			return push.GlobalApplePushClient, nil
		case config_entries.FirebasePush:
			return push.GlobalFirebasePushClient, nil
		default:
			log.WithCtx(ctx).Warn("Push: can not match app id with anyone in config",
				zap.String("app_id", appID))
			return nil, err
		}
	} else {
		log.WithCtx(ctx).Warn("Push: can not get push client item form config by app id",
			zap.String("app_id", appID))
		return nil, err
	}
}
