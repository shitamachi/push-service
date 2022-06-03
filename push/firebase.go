package push

import (
	"context"
	"errors"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/models"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"reflect"
	"sync"
)

type FirebasePushClient struct {
	clients sync.Map
}

type MessageFirebaseItem struct {
	Title    string            `json:"title,omitempty"`
	Body     string            `json:"body,omitempty"`
	ImageURL string            `json:"image,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

func NewFirebasePushClient() *FirebasePushClient {
	return &FirebasePushClient{}
}

func InitFirebasePush(ctx context.Context, appConfig *config.AppConfig) {
	for packageName := range appConfig.FirebasePushConfig.Items {
		client, err := NewFirebasePushClientItem(ctx, appConfig, packageName)
		if err != nil {
			log.WithCtx(ctx).Error("InitFirebasePush: can not create firebase push client", zap.String("package_name", packageName))
			continue
		}
		if GlobalFirebasePushClient == nil {
			GlobalFirebasePushClient = NewFirebasePushClient()
		}
		log.WithCtx(ctx).Info("InitFirebasePush: init firebase message client successfully", zap.String("package_name", packageName))
		GlobalFirebasePushClient.clients.Store(packageName, client)
	}
}

func NewFirebasePushClientItem(ctx context.Context, appConfig *config.AppConfig, packageName string) (*messaging.Client, error) {
	configItem, ok := appConfig.FirebasePushConfig.Items[packageName]
	if !ok {
		return nil, NewWrappedError(fmt.Sprintf("init google %s push client failed", packageName), CanNotGetClientFromConfig)
	}
	opts := option.WithCredentialsJSON([]byte(configItem.ServiceAccountFileContent))
	app, err := firebase.NewApp(context.Background(), nil, opts)
	if err != nil {
		log.WithCtx(ctx).Error("NewFirebasePushClientItem: error initializing firebase app", zap.String("app", packageName), zap.Error(err))
		return nil, err
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		log.WithCtx(ctx).Error("NewFirebasePushClientItem: error initializing message client", zap.String("app", packageName), zap.Error(err))
		return nil, NewWrappedError(fmt.Sprintf("init %s push message client failed", packageName), err)
	}
	return client, nil
}

func (f *FirebasePushClient) GetClientByAppID(ctx context.Context, appID string) (interface{}, bool) {
	value, ok := f.clients.Load(appID)
	if !ok {
		log.WithCtx(ctx).Error("GetClientByAppID: can not get client from global one", zap.String("package_name", appID))
		return nil, false
	}
	return value, true
}

func (f *FirebasePushClient) Push(ctx context.Context, message *models.PushMessage) (interface{}, error) {
	if message == nil {
		log.WithCtx(ctx).Error("Firebase Push: get param message is nil")
		return nil, errors.New("message is nil")
	}
	// get push message client
	value, ok := f.GetClientByAppID(ctx, message.GetAppId())
	if !ok || value == nil {
		log.WithCtx(ctx).Error("Firebase Push: can not get push client, value is nil or get operation not ok", zap.String("app", message.GetAppId()))
		return nil, NewWrappedError(fmt.Sprintf("get %s push client failed", message.GetAppId()), CanNotGetPushClient)
	}
	client, ok := value.(*messaging.Client)
	if !ok {
		log.WithCtx(ctx).Error("Firebase Push: got firebase client value from global instance, but convert to *messaging.Client failed",
			zap.String("app", message.GetAppId()),
			zap.String("type", reflect.TypeOf(client).String()))
		return nil, NewWrappedError("can not convert client value to *messaging.Client", ConvertToSpecificPlatformClientFailed)
	}

	msg, ok := message.Build(ctx).(*messaging.Message)
	if !ok {
		log.WithCtx(ctx).Error("Firebase Push: got message ok, but convert to *messaging.Message failed",
			zap.String("app", message.GetAppId()),
			zap.String("type", reflect.TypeOf(msg).String()))
		return nil, NewWrappedError("can not convert message to firebase *messaging.Message", ConvertToSpecificPlatformMessageFailed)
	}
	res, err := client.SendAll(ctx, []*messaging.Message{msg})

	if err != nil {
		log.WithCtx(ctx).Error("FirebasePush: send push request to firebase failed",
			zap.String("app", message.GetAppId()),
			zap.Any("message", message),
			zap.Error(err),
		)
		return nil, err
	}

	if res.SuccessCount <= 0 {
		log.WithCtx(ctx).Error("FirebasePush: no message send to firebase successfully",
			zap.Int("success_count", res.SuccessCount),
			zap.Int("failure_count", res.FailureCount),
			zap.Any("response", res.Responses),
		)
		return res, fmt.Errorf("FirebasePush: no message send to firebase successfully")
	}

	log.WithCtx(ctx).Debug("FirebasePush: send message to firebase for push successfully",
		zap.Int("success_count", res.SuccessCount),
		zap.Int("failure_count", res.FailureCount),
		zap.Any("response", res.Responses),
	)

	return res, nil
}
