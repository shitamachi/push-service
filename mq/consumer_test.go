package mq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/redisqueue/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"math/rand"
	"testing"
	"time"
)

const (
	testStreamKey       = "test_stream"
	testGroupKey        = "test_group"
	addTestMessageCount = 100
)

var atomicCounter = atomic.NewInt32(addTestMessageCount)

func TestInitConsumer(t *testing.T) {
	shutdownCh := make(chan struct{})
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)

	ctx := &api.Context{
		AppContext: &api.AppContext{
			Logger:      logger,
			RedisClient: redisClient,
		},
	}

	p, err := InitProducer(ctx, redisClient)
	assert.NoError(t, err)

	type args struct {
		ctx          context.Context
		redisClient  *redis.Client
		logger       *zap.Logger
		stream       string
		group        string
		consumerFunc redisqueue.ConsumerFunc
		producerFun  func(context.Context, *redisqueue.Producer) error
	}
	tests := []struct {
		name    string
		args    args
		want    *redisqueue.Consumer
		wantErr bool
	}{
		{
			name: "test consumer message",
			args: args{
				ctx:         ctx,
				redisClient: redisClient,
				logger:      logger,
				stream:      testStreamKey,
				group:       testGroupKey,
				consumerFunc: func(ctx context.Context, message *redisqueue.Message) error {
					log.WithCtx(ctx).Info("start to consumer message")

					time.Sleep(time.Millisecond * 500)

					n := rand.Intn(100)
					if n < 10 {
						log.WithCtx(ctx).Info("failed to consumer message")
						return fmt.Errorf("got error when consum message value=%d", n)
					}

					log.WithCtx(ctx).Info("finish to consumer message")

					atomicCounter.Dec()
					if atomicCounter.Load() <= 0 {
						shutdownCh <- struct{}{}
					}

					return nil
				},
				producerFun: func(ctx context.Context, producer *redisqueue.Producer) error {
					for i := 0; i < 100; i++ {
						err := producer.Enqueue(&redisqueue.Message{
							Stream: testStreamKey,
							Values: map[string]interface{}{
								"message": fmt.Sprintf("hello world %d", i),
							},
						})
						if err != nil {
							return err
						}
					}
					return nil
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := InitConsumer(tt.args.ctx, tt.args.redisClient, tt.args.logger, tt.args.stream, tt.args.group, tt.args.consumerFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitConsumer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			go func() {
				err := tt.args.producerFun(tt.args.ctx, p)
				if err != nil {
					log.WithCtx(tt.args.ctx).Error("failed to produce message", zap.Error(err))
				}
			}()
			go func() {
				<-shutdownCh
				close(shutdownCh)
				consumer.Shutdown()
				logger.Info("shutdown consumer")
			}()
			consumer.Run()
		})
	}
}
