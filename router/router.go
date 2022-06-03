package router

import (
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/handler"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func InitRouter(config *config.AppConfig, appCtx *api.AppContext) *gin.Engine {
	if config.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()
	ctx := api.Context{
		AppContext: appCtx,
	}

	r.POST("/v1/batch_push_messages_async", ctx.WrapperGinHandleFunc(handler.BatchPushMessageAsync))
	r.POST("/v1/push_messages_for_all", ctx.WrapperGinHandleFunc(handler.PushMessageForAllSpecificClient))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//pprof
	pprof.RouteRegister(r.Group(""))
	return r
}
