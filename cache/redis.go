package cache

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"go.uber.org/zap"
	"time"
)

var Client *redis.Client

func InitRedis() {

	var cli *redis.Client
	var err error
	var conf = config.GlobalConfig.CacheConfig

	cli = redis.NewClient(&redis.Options{
		Addr:         conf.RedisAddr,
		Password:     conf.RedisPassword,
		DB:           conf.RedisDB,
		ReadTimeout:  time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(conf.WriteTimeout) * time.Second,
	})

	_, err = cli.Ping(context.Background()).Result()
	if err != nil {
		log.Logger.Fatal("get connection to redis failed", zap.Error(err))
	}

	log.Logger.Info("init cache redis client successfully")

	Client = cli
}
