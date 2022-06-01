package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	"github.com/mitchellh/mapstructure"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/models"
	"github.com/shitamachi/push-service/mq"
	"github.com/shitamachi/push-service/push"
	"go.uber.org/zap"
	"reflect"
	"sync"
	"time"
)

const (
	PushMessageStreamKey = "push_message_stream"
	PushMessageGroupKey  = "push_message_group"
)

var initOnce sync.Once

func InitSendPushQueue(ctx context.Context) {
	initOnce.Do(func() {
		err := CreateMQGroup(ctx)
		if err != nil {
			log.Logger.Error("InitSendPushQueue: CreateMQGroup failed", zap.Error(err))
			return
		}
		log.Logger.Info("InitSendPushQueue: CreateMQGroup successfully")

		// create consumer
		for i := 0; i < config.GlobalConfig.Mq.InitCreatedConsumerCount; i++ {
			go func(i int) {
				NewConsumer(
					ctx,
					fmt.Sprintf("push_message_consumer_%d_%d", config.GlobalConfig.WorkerID, i),
					processPushMessage,
				)
			}(i)
		}

		// handle pending message
		go func() {
			for t := range time.Tick(time.Duration(config.GlobalConfig.Mq.RecoverMessageDuration) * time.Millisecond) {
				log.Logger.Debug("ClaimPendingMessage: run claim pending message task", zap.Time("t", t))
				err := mq.ClaimPendingMessage(ctx, PushMessageStreamKey, PushMessageGroupKey)
				if err != nil {
					log.Logger.Error("ClaimPendingMessage: exit claim pending message", zap.Error(err))
					return
				}
			}
		}()
	})
}

type PushStreamMessage struct {
	models.BaseMessage `mapstructure:",squash"`
	AppId              string `json:"app_id" mapstructure:"app_id"`
	Token              string `json:"token" mapstructure:"token"`
	UserId             string `json:"user_id" mapstructure:"user_id"`
	ActionId           string `json:"action_id" mapstructure:"action_id"`
}

func CreateMQGroup(ctx context.Context) error {

	var (
		isGroupCreated    bool
		createGroupResult string
	)
	groups, _ := cache.Client.XInfoGroups(ctx, PushMessageStreamKey).Result()
	if groups == nil || len(groups) <= 0 {
		isGroupCreated = false
	} else {
		for _, group := range groups {
			if group.Name == PushMessageGroupKey {
				isGroupCreated = true
				break
			}
		}
	}

	if !isGroupCreated {
		res, err := cache.Client.XGroupCreateMkStream(ctx, PushMessageStreamKey, PushMessageGroupKey, "$").Result()
		if err != nil {
			log.Logger.Error("CreateMQGroup: failed to creat redis group", zap.Error(err), zap.String("group", PushMessageGroupKey))
			return err
		}
		createGroupResult = res
	} else {
		createGroupResult = fmt.Sprintf("group %s already created", PushMessageGroupKey)
	}

	log.Logger.Info("CreateMQGroup: create MQ group successfully",
		zap.String("res", createGroupResult),
		zap.String("group", PushMessageGroupKey),
	)
	return nil
}

func AddMessageToStream(ctx context.Context, stream string, values interface{}) error {
	result, err := cache.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		Values: values,
	}).Result()
	if err != nil {
		log.Logger.Error("add item to stream failed",
			zap.String("stream", stream),
			zap.Any("values", values),
		)
		return err
	}

	log.Logger.Info("add item to steam successfully",
		zap.String("stream", stream),
		zap.String("res", result),
	)

	return nil
}

func NewConsumer(ctx context.Context,
	consumerName string,
	processFunc func(context.Context, *redis.XMessage) error,
) {

	var (
		lastId       = "0-0"
		checkBackLog = true
	)

	log.Logger.Info("Consumer starting...", zap.String("consumer_name", consumerName))

	for {
		// Pick the ID based on the iteration: the first time we want to
		// read our pending messages, in case we crashed and are recovering.
		// Once we consume our history, we can start getting new messages.
		var consumerId string
		if checkBackLog {
			consumerId = lastId
		} else {
			consumerId = ">"
		}

		streams, err := cache.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Streams:  []string{PushMessageStreamKey, consumerId},
			Group:    PushMessageGroupKey,
			Consumer: consumerName,
			Count:    int64(config.GlobalConfig.Mq.OnceReadMessageCount), // once consumer one message
			Block:    2000,
		}).Result()
		if err != nil {
			// may can ignore timeout error
			//log.Logger.Debug("Consumer: consumer redis group timeout",
			//	zap.String("consumer_name", consumerName),
			//	zap.String("group", PushMessageGroupKey),
			//	zap.Error(err))
			continue
		}

		// If we receive an empty reply, it means we were consuming our history
		// and that the history is now empty. Let's start to consume new messages.
		if streams == nil || len(streams) <= 0 || len(streams[0].Messages) <= 0 {
			checkBackLog = false
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				actionId, ok := message.Values["action_id"].(string)
				if !ok {
					log.Logger.Warn("Consumer: get action id failed",
						zap.String("type", reflect.TypeOf(actionId).String()))
				}

				err = processFunc(ctx, &message)

				if err != nil {
					_, err = cache.Client.XDel(ctx, PushMessageStreamKey, message.ID).Result()
					if err != nil {
						log.Logger.Error("Consumer: consume message failed, should not retry, del message failed",
							zap.String("action_id", actionId),
							zap.Error(err),
						)
						continue
					}
					log.Logger.Debug("Consumer: del message successfully", zap.Any("message", message))
				} else {
					count, outAckErr := cache.Client.XAck(ctx, PushMessageStreamKey, PushMessageGroupKey, message.ID).Result()
					if outAckErr != nil {
						log.Logger.Error("Consumer: ack message failed",
							zap.String("action_id", actionId),
							zap.String("consumer_name", consumerName),
							zap.Error(outAckErr),
							zap.Any("message", message))
						continue
					}

					log.Logger.Debug("Consumer: consumer message successfully",
						zap.String("action_id", actionId),
						zap.String("consumer_name", consumerName),
						zap.Int64("ack_count", count),
						zap.Any("message", message),
					)
				}
			}
		}
	}
}

func processPushMessage(ctx context.Context, message *redis.XMessage) (err error) {
	var psm = new(PushStreamMessage)
	psm.Data = psm.BaseMessage.DecodeData()
	err = mapstructure.Decode(message.Values, psm)
	if err != nil {
		log.Logger.Error("Push: can not decode map to struct", zap.Any("message", message))
		return fmt.Errorf("can not decode map to struct PushStreamMessage")
	}

	client, err := getPushClientByAppId(psm.AppId)
	if err != nil {
		log.Logger.Warn("Push: can not get push message client by app id", zap.String("app_id", psm.AppId))
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
				log.Logger.Warn("Push: send push message timeout")
				return err
			case err != nil:
				// should not retry
				log.Logger.Info("Push: push notification failed",
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
			log.Logger.Info("Push: failed to push message, will retry push again",
				zap.String("app_id", psm.AppId),
				zap.Error(err),
			)
		})

	if err != nil {
		log.Logger.Error("Push: failed to push message",
			zap.String("app_id", psm.AppId),
			zap.Error(err),
			zap.Any("message", psm.BaseMessage),
		)
	}

	return
}

func getPushClientByAppId(appID string) (push.Pusher, error) {
	err := fmt.Errorf("can not get push client item form config by app id=\"%s\"", appID)

	if len(appID) <= 0 {
		return nil, err
	}

	item, ok := config.GlobalConfig.ClientConfig[appID]
	if ok {
		switch item.PushType {
		case config_entries.ApplePush:
			return push.GlobalApplePushClient, nil
		case config_entries.FirebasePush:
			return push.GlobalFirebasePushClient, nil
		default:
			log.Logger.Warn("Push: can not match app id with anyone in config",
				zap.String("app_id", appID))
			return nil, err
		}
	} else {
		log.Logger.Warn("Push: can not get push client item form config by app id",
			zap.String("app_id", appID))
		return nil, err
	}
}
