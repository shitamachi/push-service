package cache

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/config/config_entries"
	"time"
)

func InitRedis(ctx context.Context, conf config_entries.CacheConfig) (*redis.Client, error) {

	var (
		cli *redis.Client
		err error
	)

	cli = redis.NewClient(&redis.Options{
		Addr:         conf.RedisAddr,
		Password:     conf.RedisPassword,
		DB:           conf.RedisDB,
		ReadTimeout:  time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(conf.WriteTimeout) * time.Second,
	})

	_, err = cli.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}

	return cli, err
}
