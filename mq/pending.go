package mq

import (
	"context"
	"fmt"
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
		log.Logger.Error("Claim: get pending list failed",
			zap.String("stream", stream),
			zap.String("group", group),
		)
		return nil, err
	}
	return pendingList, nil
}

func ClaimPendingMessage(ctx context.Context, stream, group string) error {
	pendingMessages, err := GetPendingMessages(ctx, stream, group)
	if err != nil {
		log.Logger.Error("Claim: failed to get pending messages",
			zap.String("stream", stream),
			zap.String("group", group),
		)
		return err
	} else if len(pendingMessages) <= 0 {
		log.Logger.Debug("Claim: no pending message need to claim")
		return nil
	}

	claimConsumer, err := GetConsumer(ctx, stream, group)
	if err != nil {
		log.Logger.Error("Claim: failed to get consumer",
			zap.String("stream", stream), zap.String("group", group))
		return err
	}

	for _, message := range pendingMessages {
		if err = processClaimMessage(ctx, stream, group, claimConsumer, &message); err != nil {
			log.Logger.Error("Claim: claim message failed", zap.Error(err),
				zap.Any("message_info", message),
				zap.String("claim_consumer", claimConsumer.Name),
				zap.String("stream", stream),
				zap.String("group", group),
			)
		}
	}

	return nil
}

func processClaimMessage(
	ctx context.Context,
	stream, group string,
	consumer *redis.XInfoConsumer,
	message *redis.XPendingExt,
) error {

	if message.Idle < constIdleTimeout {
		return nil
	}

	_, err := cache.Client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer.Name,
		MinIdle:  constIdleTimeout,
		Messages: []string{message.ID},
	}).Result()
	if err != nil && err != redis.Nil {
		log.Logger.Error("Claim: xclaim message to active consumer failed",
			zap.Error(err),
			zap.Any("message_info", message),
			zap.String("claim_consumer", consumer.Name),
		)
		return err
	} else if err == redis.Nil || int(message.RetryCount+1) > config.GlobalConfig.Mq.MaxRetryCount {
		// condition 1:
		// If the Redis nil error is returned, it means that
		// the message no longer exists in the stream.
		// However, it is still in a pending state. This
		// could happen if a message was claimed by a
		// consumer, that consumer died, and the message
		// gets deleted (either through a XDEL call or
		// through MAXLEN). Since the message no longer
		// exists, the only way we can get it out of the
		// pending state is to acknowledge it.
		// condition 2:
		// If the message has been retried too many times, then we will ack it.
		_, err := cache.Client.XAck(ctx, stream, group, consumer.Name, message.ID).Result()
		if err != nil {
			log.Logger.Error("Claim: error acknowledging after failed claim for stream and message",
				zap.Error(err),
				zap.Any("message_info", message),
				zap.String("claim_consumer", consumer.Name),
				zap.Bool("redis_err_is_nil", err == redis.Nil),
				zap.Int64("current_retry_count", message.RetryCount+1),
				zap.Bool("reach_max_retry_count", int(message.RetryCount+1) > config.GlobalConfig.Mq.MaxRetryCount),
			)
		}
		// we don't return error here, because we want to continue to claim other messages
		return nil
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
		log.Logger.Error("Claim: failed to get consumers",
			zap.String("stream", stream),
			zap.String("group", group),
			zap.Error(err),
		)
		return
	}

	if len(consumers) <= 0 {
		log.Logger.Warn("Claim: can not get consumers",
			zap.String("stream", stream), zap.String("group", group),
		)
		return nil, fmt.Errorf("get consumer result is empty")
	}

	sort.Slice(consumers, func(i, j int) bool {
		return consumers[i].Pending < consumers[j].Pending
	})

	return &consumers[0], nil
}
