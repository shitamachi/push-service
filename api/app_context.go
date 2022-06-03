package api

import (
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/redisqueue/v2"
	"go.uber.org/zap"
	"time"
)

type AppContext struct {
	Config      *config.AppConfig
	Logger      *zap.Logger
	RedisClient *redis.Client
	Db          *ent.Client
	Consumer    *redisqueue.Consumer
	Producer    *redisqueue.Producer
}

func NewAppContext(config *config.AppConfig, logger *zap.Logger, redisClient *redis.Client, db *ent.Client, consumer *redisqueue.Consumer, producer *redisqueue.Producer) *AppContext {
	return &AppContext{Config: config, Logger: logger, RedisClient: redisClient, Db: db, Consumer: consumer, Producer: producer}
}

func (a AppContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (a AppContext) Done() <-chan struct{} {
	return nil
}

func (a AppContext) Err() error {
	return nil
}

func (a AppContext) Value(key any) any {
	switch key.(type) {
	case config.SetConfigToContextKey:
		return a.Config
	case log.SetLoggerToContextKey:
		return a.Logger
	default:
		return nil
	}
}
