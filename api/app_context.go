package api

import (
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/redisqueue/v2"
	"go.uber.org/zap"
)

type AppContext struct {
	Logger      *zap.Logger
	RedisClient *redis.Client
	Db          *ent.Client
	Consumer    *redisqueue.Consumer
	Producer    *redisqueue.Producer
}

func NewAppContext(logger *zap.Logger, redisClient *redis.Client, db *ent.Client, consumer *redisqueue.Consumer, producer *redisqueue.Producer) *AppContext {
	return &AppContext{Logger: logger, RedisClient: redisClient, Db: db, Consumer: consumer, Producer: producer}
}
