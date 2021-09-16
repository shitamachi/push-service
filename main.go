package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/cache"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/db"
	_ "github.com/shitamachi/push-service/docs" // docs is generated by Swag CLI, you have to import it.
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/push-service/handler"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/push"
	"github.com/shitamachi/push-service/service"
	"github.com/swaggo/files"       // swagger embed files
	"github.com/swaggo/gin-swagger" // gin-swagger middleware
	"go.uber.org/zap"
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

// @host petstore.swagger.io
// @BasePath /v1
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

	gin.SetMode(gin.DebugMode)
	r := gin.New()
	r.POST("/v1/push_messages", api.WrapperGinHandleFunc(handler.PushMessage))
	r.POST("/v1/batch_push_messages", api.WrapperGinHandleFunc(handler.BatchPushMessage))
	r.POST("/v1/push_messages_for_all", api.WrapperGinHandleFunc(handler.PushMessageForAllSpecificClient))
	//todo
	//api.HandleFunc("/v1/set_token", handler.SetUserPushToken)
	//api.HandleFunc("/v1/batch_push_messages_async", handler.BatchPushMessageInQueue)
	// health
	//api.HandleFunc("/health", handler.Health)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Sugar.Fatal(r.Run(fmt.Sprintf(":%d", config.GlobalConfig.Port)))
}
