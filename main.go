package main

import (
	"context"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/db"
	_ "github.com/shitamachi/push-service/docs" // docs is generated by Swag CLI, you have to import it.
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/mq"
	"github.com/shitamachi/push-service/push"
	"github.com/shitamachi/push-service/router"
	"github.com/shitamachi/push-service/service"
	"github.com/shitamachi/push-service/utils"
	"github.com/shitamachi/redisqueue/v2"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title Push Service API
// @version 1.0
// @description push service http server
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost
// @BasePath /v1
func main() {
	ctx := context.Background()
	// init config
	appConfig := config.InitConfig()
	//init logger
	logger, err := log.InitLogger(appConfig)
	utils.CheckErr(err)
	ctx = log.SetLoggerToContext(ctx, logger)
	// init db
	client := db.InitDB(appConfig)
	defer func(client *ent.Client) {
		err := client.Close()
		utils.CheckErr(err)
	}(client)
	// init redis
	redisClient, err := cache.InitRedis(ctx, appConfig.CacheConfig)
	utils.CheckErr(err)
	// init push client
	push.InitApplePush(ctx, appConfig)
	push.InitFirebasePush(ctx, appConfig)
	// init message producer
	producer, err := mq.InitProducer(ctx, redisClient)
	utils.CheckErr(err)
	// init message consumer
	consumer, err := mq.InitConsumer(
		ctx,
		redisClient,
		logger,
		mq.PushMessageStreamKey,
		mq.PushMessageGroupKey,
		service.ProcessPushMessage,
	)
	utils.CheckErr(err)

	appContext := api.NewAppContext(appConfig, logger, redisClient, client, consumer, producer)

	// init router
	r := router.InitRouter(appConfig, appContext)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", appConfig.Port),
		Handler: r,
	}

	// run consumer and server
	run(logger, srv, consumer)
}

func run(logger *zap.Logger, srv *http.Server, consumer *redisqueue.Consumer) {
	go func() {
		logger.Info("consumer message start")
		consumer.Run()
		logger.Info("consumer message stopped")
	}()
	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("ListenAndServe failed",
				zap.String("addr", srv.Addr),
				zap.Error(err),
			)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}
