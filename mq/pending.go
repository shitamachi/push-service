package mq

import (
	"context"
	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/log"
	"go.uber.org/zap"
	"math/rand"
	"time"
)

// GetPendingMessages TODO 支持多 stream group
func GetPendingMessages(ctx context.Context, stream, group string) ([]redis.XPendingExt, error) {
	pendingList, err := cache.Client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Idle:   18000 * time.Second, //todo 硬编码
		Start:  "-",
		End:    "+",
	}).Result()
	if err != nil {
		log.Logger.Error("GetPendingMessages: get pending list failed", zap.String("stream", stream), zap.String("group", group))
		return nil, err
	}
	return pendingList, nil
}

// GetActiveConsumers return not pending message consumers
func GetActiveConsumers(ctx context.Context, stream, group string) (consumers []redis.XInfoConsumer, err error) {
	xInfoConsumers, err := cache.Client.XInfoConsumers(ctx, stream, group).Result()
	if err != nil {
		log.Logger.Error("GetActiveConsumers: can not execute xinfo xInfoConsumers cmd for get pending msg xInfoConsumers", zap.Error(err))
		return nil, err
	}
	for _, consumer := range xInfoConsumers {
		if consumer.Pending == 0 {
			consumers = append(consumers, consumer)
		}
	}
	return
}

func ClaimPendingMessage(ctx context.Context, stream, group string) error {
	pendingMessages, err := GetPendingMessages(ctx, stream, group)
	if err != nil {
		log.Logger.Error("ClaimPendingMessage: failed to get pending messages", zap.String("stream", stream), zap.String("group", group))
		return err
	} else if len(pendingMessages) <= 0 {
		log.Logger.Debug("ClaimPendingMessage: no pending message need to claim")
		return nil
	}

	consumers, err := GetActiveConsumers(ctx, stream, group)
	if err != nil {
		log.Logger.Error("ClaimPendingMessage: failed to get active consumer list", zap.String("stream", stream), zap.String("group", group))
		return err
	}
	if len(consumers) <= 0 {
		log.Logger.Warn("ClaimPendingMessage: get active consumer result is empty")
		//return fmt.Errorf("ClaimPendingMessage: get active consumer result is empty")
		return nil
	}

	for _, message := range pendingMessages {
		var claimPendingMessage = func() error {
			claimConsumer := consumers[rand.Intn(len(consumers))].Name
			_, err := cache.Client.XClaim(ctx, &redis.XClaimArgs{
				Stream:   stream,
				Group:    group,
				Consumer: claimConsumer,
				MinIdle:  18000 * time.Second,
				Messages: []string{message.ID},
			}).Result()
			if err != nil {
				log.Logger.Error("ClaimPendingMessage: xclaim message to active consumer failed",
					zap.Any("message_info", message),
					zap.String("claim_consumer", claimConsumer),
				)
				return err
			}
			return nil
		}

		if err = claimPendingMessage(); err != nil {
			err := backoff.RetryNotify(
				claimPendingMessage,
				backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5),
				func(err error, duration time.Duration) {
					log.Logger.Error("ClaimPendingMessage: claim message failed", zap.Error(err), zap.Duration("delay_duration", duration))
				},
			)
			if err != nil {
				log.Logger.Error("ClaimPendingMessage: backoff retry claim pending message failed, try to del it", zap.Error(err))
				_, err := cache.Client.XDel(ctx, stream, message.ID).Result()
				if err != nil {
					log.Logger.Error("ClaimPendingMessage: maximum number of retries to claim message, try del it failed",
						zap.Error(err),
						zap.Any("message_info", message),
					)
					return err
				}
				//return err
			}
		}
	}

	return nil
}