package mq

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/redisqueue/v2"
)

func InitProducer(ctx context.Context, client *redis.Client) (*redisqueue.Producer, error) {
	p, err := redisqueue.NewProducerWithOptions(&redisqueue.ProducerOptions{
		Ctx:                  ctx,
		StreamMaxLength:      10000,
		ApproximateMaxLength: true,
		RedisClient:          client,
	})
	return p, err
}
