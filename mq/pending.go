package mq

import (
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"go.uber.org/zap"
	"sort"
	"time"
)

const constIdleTimeout = time.Second * 30

// GetPendingMessages TODO 支持多 stream group
func GetPendingMessages(ctx context.Context, stream, group string) ([]redis.XPendingExt, error) {
	pendingList, err := cache.Client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		log.Logger.Error("GetPendingMessages: get pending list failed", zap.String("stream", stream), zap.String("group", group))
		return nil, err
	}
	return pendingList, nil
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

	claimConsumer, err := GetConsumer(ctx, stream, group)
	if err != nil {
		log.Logger.Error("ClaimPendingMessage: failed to get consumer",
			zap.String("stream", stream), zap.String("group", group))
		return err
	}

	for _, message := range pendingMessages {
		var claimPendingMessage = func() error {
			if message.Idle < constIdleTimeout {
				return nil
			}
			_, err := cache.Client.XClaim(ctx, &redis.XClaimArgs{
				Stream:   stream,
				Group:    group,
				Consumer: claimConsumer.Name,
				MinIdle:  constIdleTimeout,
				Messages: []string{message.ID},
			}).Result()
			if err != nil {
				log.Logger.Error("ClaimPendingMessage: xclaim message to active consumer failed",
					zap.Error(err),
					zap.Any("message_info", message),
					zap.String("claim_consumer", claimConsumer.Name),
				)
				return err
			}
			return nil
		}

		if err = claimPendingMessage(); err != nil {
			err := backoff.RetryNotify(
				claimPendingMessage,
				backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3),
				func(err error, duration time.Duration) {
					log.Logger.Error("ClaimPendingMessage: claim message failed", zap.Error(err), zap.Duration("delay_duration", duration))
				},
			)
			if err != nil {
				log.Logger.Error("ClaimPendingMessage: backoff retry claim pending message failed, try to del it",
					zap.Error(err),
					zap.Any("message_info", message),
				)
			}
		}

		if int(message.RetryCount) > config.GlobalConfig.Mq.MaxRetryCount {
			res, err := cache.Client.XAck(ctx, stream, group, message.ID).Result()
			log.Logger.Debug("ClaimPendingMessage: xack info ", zap.Any("xack_info", res))
			if err != nil {
				log.Logger.Error("ClaimPendingMessage: maximum number of retries to claim message, try ack it failed",
					zap.Error(err),
					zap.Any("message_info", message),
				)
				return err
			}
		}
	}

	return nil
}

// GetConsumer return low pressure consumer list
func GetConsumer(ctx context.Context, stream, group string) (
	consumer *redis.XInfoConsumer,
	err error,
) {
	consumers, err := cache.Client.XInfoConsumers(ctx, stream, group).Result()
	if err != nil {
		log.Logger.Error("Reclaim: failed to get consumers",
			zap.String("stream", stream),
			zap.String("group", group),
			zap.Error(err),
		)
		return
	}

	if len(consumers) <= 0 {
		log.Logger.Warn("Reclaim: can not get consumers",
			zap.String("stream", stream), zap.String("group", group),
		)
		return nil, fmt.Errorf("get consumer result is empty")
	}

	sort.Slice(consumers, func(i, j int) bool {
		return consumers[i].Pending < consumers[j].Pending
	})

	return &consumers[0], nil
}
