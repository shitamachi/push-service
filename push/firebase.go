package push

import (
	"context"
	"errors"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
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

func InitFirebasePush() {
	for packageName, _ := range config.GlobalConfig.FirebasePushConfig.Items {
		client, err := NewFirebasePushClientItem(packageName)
		if err != nil {
			log.Logger.Error("InitFirebasePush: can not create firebase push client", zap.String("package_name", packageName))
			continue
		}
		if GlobalFirebasePushClient == nil {
			GlobalFirebasePushClient = NewFirebasePushClient()
		}
		log.Logger.Info("InitFirebasePush: init firebase message client successfully", zap.String("package_name", packageName))
		GlobalFirebasePushClient.clients.Store(packageName, client)
	}
}

func NewFirebasePushClientItem(packageName string) (*messaging.Client, error) {
	configItem, ok := config.GlobalConfig.FirebasePushConfig.Items[packageName]
	if !ok {
		return nil, CanNotGetClientFromConfig
	}
	opts := option.WithCredentialsJSON([]byte(configItem.ServiceAccountFileContent))
	app, err := firebase.NewApp(context.Background(), nil, opts)
	if err != nil {
		log.Logger.Error("NewFirebasePushClientItem: error initializing firebase app", zap.Error(err))
		return nil, err
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		log.Logger.Error("NewFirebasePushClientItem: error initializing message client", zap.Error(err))
		return nil, err
	}
	return client, nil
}

func (f *FirebasePushClient) GetClientByAppID(appID string) (interface{}, bool) {
	value, ok := f.clients.Load(appID)
	if !ok {
		log.Logger.Error("GetClientByAppID: can not get client from global one", zap.String("package_name", appID))
		return nil, false
	}
	return value, true
}

func (f *FirebasePushClient) Push(ctx context.Context, appId, message, token string, data map[string]string) (interface{}, error) {
	reqMessage := MessageFirebaseItem{}
	err := jsoniter.Unmarshal([]byte(message), &reqMessage)
	if err != nil {
		log.Logger.Error("Firebase Push: can not unmarshal the request message",
			zap.Any("message", message),
			zap.Error(err),
		)
		return nil, err
	}

	// get push message client
	value, ok := f.GetClientByAppID(appId)
	if !ok || value == nil {
		log.Logger.Error("Firebase Push: can not get push client, value is nil or get operation not ok")
		return nil, errors.New("can not get client")
	}
	client, ok := value.(*messaging.Client)
	if !ok {
		log.Logger.Error("Push: got firebase client value from global instance, but convert to *messaging.Client failed",
			zap.String("type", reflect.TypeOf(client).String()))
		return nil, errors.New("can not convert client value to *messaging.Client")
	}

	res, err := client.SendAll(ctx, []*messaging.Message{
		{
			Notification: &messaging.Notification{
				Title:    reqMessage.Title,
				Body:     reqMessage.Body,
				ImageURL: reqMessage.ImageURL,
			},
			Data:  reqMessage.Data,
			Token: token,
		},
	})

	if err != nil {
		log.Logger.Error("FirebasePush: send push request to firebase failed", zap.Error(err))
		return nil, err
	}

	if res.SuccessCount <= 0 {
		log.Logger.Error("FirebasePush: no message send to firebase successfully",
			zap.Int("success_count", res.SuccessCount),
			zap.Int("failure_count", res.FailureCount),
			zap.Any("response", res.Responses),
		)
		return res, fmt.Errorf("FirebasePush: no message send to firebase successfully")
	}

	log.Logger.Info("FirebasePush: send message to firebase for push successfully",
		zap.Int("success_count", res.SuccessCount),
		zap.Int("failure_count", res.FailureCount),
		zap.Any("response", res.Responses))

	return res, nil
}
