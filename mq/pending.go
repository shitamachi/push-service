package mq

import (
	"context"
	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"go.uber.org/zap"
	"math"
	"math/rand"
	"sort"
	"time"
)

// GetPendingMessages TODO 支持多 stream group
func GetPendingMessages(ctx context.Context, stream, group string) ([]redis.XPendingExt, error) {
	pendingList, err := cache.Client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		log.Logger.Error("GetPendingMessages: get pending list failed", zap.String("stream", stream), zap.String("group", group))
		return nil, err
	}
	return pendingList, nil
}

// GetLowPressureConsumers return low pressure consumer list
func GetLowPressureConsumers(ctx context.Context, stream, group string) (consumers []redis.XInfoConsumer, err error) {
	consumers, err = cache.Client.XInfoConsumers(ctx, stream, group).Result()
	if err != nil {
		log.Logger.Error("GetLowPressureConsumers: failed to get consumers",
			zap.String("stream", stream), zap.String("group", group), zap.Error(err),
		)
		return nil, err
	}

	if len(consumers) <= 0 {
		log.Logger.Warn("GetLowPressureConsumers: can not get consumers",
			zap.String("stream", stream), zap.String("group", group),
		)
		return
	}

	sort.Slice(consumers, func(i, j int) bool {
		return consumers[i].Pending < consumers[j].Pending
	})

	// half of the sorted consumers
	end := int(math.Ceil(float64(len(consumers) / 2)))
	return consumers[:end], nil
}

func getActiveConsumer(ctx context.Context, stream, group string) (consumers []redis.XInfoConsumer, err error) {
	activeConsumers, err := cache.Client.XInfoConsumers(ctx, stream, group).Result()
	if err != nil {
		log.Logger.Error("getActiveConsumer: failed to get active consumers currently", zap.Error(err))
		return nil, err
	}

	if len(activeConsumers) <= 0 {
		log.Logger.Warn("getActiveConsumer: no active consumers currently",
			zap.String("stream", stream), zap.String("group", group),
		)
		return nil, err
	}

	return activeConsumers, nil
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

	consumers, err := GetLowPressureConsumers(ctx, stream, group)
	if err != nil {
		log.Logger.Error("ClaimPendingMessage: failed to get active consumer list", zap.String("stream", stream), zap.String("group", group))
		return err
	} else if len(consumers) <= 0 {
		log.Logger.Info("ClaimPendingMessage: get active consumer result is empty")
		consumers, err = getActiveConsumer(ctx, stream, group)
		if err != nil {
			log.Logger.Error("ClaimPendingMessage: get random consumer failed", zap.Error(err))
			return err
		}
	}

	for _, message := range pendingMessages {
		var claimPendingMessage = func() error {
			claimConsumer := consumers[rand.Intn(len(consumers))].Name
			_, err := cache.Client.XClaim(ctx, &redis.XClaimArgs{
				Stream:   stream,
				Group:    group,
				Consumer: claimConsumer,
				MinIdle:  message.Idle,
				Messages: []string{message.ID},
			}).Result()
			if err != nil {
				log.Logger.Error("ClaimPendingMessage: xclaim message to active consumer failed",
					zap.Error(err),
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
