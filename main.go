package main

import (
	"context"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/db"
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/push-service/handler"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/push"
	"github.com/shitamachi/push-service/service"
	"go.uber.org/zap"
	"net/http"
)

func main() {
	//init logger
	log.InitLogger()

	// init db
	client, err := db.InitDB()
	if err != nil {
		panic(err)
	}
	defer func(client *ent.Client) {
		err := client.Close()
		if err != nil {
			log.Logger.Fatal("close db client failed", zap.Error(err))
		}
	}(client)

	// init redis
	cache.InitRedis()

	push.InitApplePush()
	push.InitFirebasePush()

	// init queue
	ctx := context.Background()
	service.InitSendPushQueue(ctx)

	api.HandleFunc("/set_token", handler.SetUserPushToken)
	api.HandleFunc("/push_message", handler.PushMessage)
	api.HandleFunc("/bulk_push_message", handler.BatchPushMessage)
	api.HandleFunc("/bulk_push_message_queue", handler.BatchPushMessageInQueue)
	// health
	api.HandleFunc("/health", handler.Health)

	log.Sugar.Infof("init server listen port %d", config.GlobalConfig.Port)
	log.Sugar.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.GlobalConfig.Port), nil))
}
