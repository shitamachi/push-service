package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/mitchellh/mapstructure"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/push"
	"go.uber.org/zap"
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

		for i := 0; i < 5; i++ {
			go func(i int) {
				CreateConsumer(ctx, fmt.Sprintf("push_message_consumer_%d_%d", config.GlobalConfig.WorkerID, i), processPushMessage)
			}(i)
		}
	})
}

type PushStreamMessage struct {
	Message string `json:"message" mapstructure:"message"`
	AppId   string `json:"app_id" mapstructure:"app_id"`
	Token   string `json:"token" mapstructure:"token"`
	UserId  string `json:"user_id" mapstructure:"user_id"`
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

func AddMessageToStream(ctx context.Context, stream string, values map[string]interface{}) error {
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

func CreateConsumer(ctx context.Context, consumerName string, processFunc func(context.Context, *redis.XMessage) error) {

	var (
		lastId       = "0-0"
		checkBackLog = true
	)

	log.Logger.Info("Consumer starting...", zap.String("consumer_name", consumerName))

	for {
		// Pick the ID based on the iteration: the first time we want to
		// read our pending messages, in case we crashed and are recovering.
		// Once we consumer our history, we can start getting new messages.
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
			Count:    1, // once consumer one message
			Block:    2000,
		}).Result()
		if err != nil {
			log.Logger.Debug("consumer redis group timeout",
				zap.String("consumer_name", consumerName),
				zap.String("group", PushMessageGroupKey),
				zap.Error(err))
			continue
		}

		// If we receive an empty reply, it means we were consuming our history
		// and that the history is now empty. Let's start to consume new messages.
		if streams == nil || len(streams) <= 0 || len(streams[0].Messages) <= 0 {
			checkBackLog = false
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				err = processFunc(ctx, &message)

				count, outAckErr := cache.Client.XAck(ctx, PushMessageStreamKey, PushMessageGroupKey, message.ID).Result()
				if outAckErr != nil {
					log.Logger.Error("ack message failed",
						zap.String("consumer_name", consumerName),
						zap.Error(outAckErr),
						zap.String("message_id", message.ID))
					continue
				}

				if err != nil {
					log.Logger.Error("consumer streams failed",
						zap.Error(err),
						zap.String("consumer_name", consumerName),
						zap.String("message_id", message.ID))
					result, err := cache.Client.XDel(ctx, PushMessageStreamKey, message.ID).Result()
					if err != nil {
						log.Logger.Error("del message failed",
							zap.String("consumer_name", consumerName),
							zap.Error(err),
							zap.String("message_id", message.ID),
							zap.Int64("del_count", result),
						)
					} else {
						log.Logger.Info("del message successfully",
							zap.String("consumer_name", consumerName),
							zap.String("message_id", message.ID),
							zap.Int64("del_count", result),
						)
					}
					continue
				} else {
					log.Logger.Info("consumer message successfully",
						zap.String("consumer_name", consumerName),
						zap.Int64("ack_count", count),
						zap.String("message_id", message.ID),
					)
				}
			}
		}
	}
}

func processPushMessage(ctx context.Context, message *redis.XMessage) error {
	var psm PushStreamMessage
	err := mapstructure.Decode(message.Values, &psm)
	if err != nil {
		log.Logger.Error("processPushMessage: can not decode map to struct")
		return fmt.Errorf("processPushMessage: can not decode map to struct")
	}
	client := getProcessPushMessageClient(psm.AppId)
	if client == nil {
		log.Logger.Error("processPushMessage: can not get push message client")
		return errors.New("can not get push message client")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = client.Push(ctx, psm.AppId, psm.Message, psm.Token, nil)
	if err != nil {
		log.Logger.Error("processPushMessage: send push failed", zap.Error(err))
		return err
	}
	return nil
}

func getProcessPushMessageClient(appID string) push.Pusher {
	if len(appID) <= 0 {
		return nil
	}

	item, ok := config.GlobalConfig.ClientConfig[appID]
	if ok {
		switch item.PushType {
		case config_entries.ApplePush:
			return push.GlobalApplePushClient
		case config_entries.FirebasePush:
			return push.GlobalFirebasePushClient
		default:
			log.Logger.Error("getProcessPushMessageClient: can not match app id with anyone in config",
				zap.String("app_id", appID))
			return nil
		}
	} else {
		log.Logger.Error("getProcessPushMessageClient: can not get push client item form config by app id",
			zap.String("app_id", appID))
		return nil
	}
}
