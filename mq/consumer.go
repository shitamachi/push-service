package mq

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/redisqueue/v2"
	"go.uber.org/zap"
	"time"
)

func InitConsumer(
	ctx context.Context,
	redisClient *redis.Client,
	logger *zap.Logger,
	stream, group string,
	consumerFunc redisqueue.ConsumerFunc,
) (*redisqueue.Consumer, error) {
	c, err := redisqueue.NewConsumerWithOptions(&redisqueue.ConsumerOptions{
		Ctx:                  ctx,
		GroupName:            group,
		VisibilityTimeout:    10 * time.Second,
		BlockingTimeout:      5 * time.Second,
		ReclaimInterval:      1 * time.Second,
		ReclaimMaxRetryCount: 5,
		BufferSize:           100,
		Concurrency:          10,
		RedisClient:          redisClient,
	})
	if err != nil {
		return c, err
	}

	c.Register(stream, consumerFunc)

	go func() {
		for err := range c.Errors {
			// handle errors accordingly
			logger.Error("Consumer: consumer error", zap.Error(err))
		}
	}()

	return c, err
}
