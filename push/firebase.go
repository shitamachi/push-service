package push

import (
	"context"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
)

var FcmClient *messaging.Client

type FirebasePushClient struct {
	FcmClient *messaging.Client
}

type MessageFirebaseItem struct {
	Title    string            `json:"title,omitempty"`
	Body     string            `json:"body,omitempty"`
	ImageURL string            `json:"image,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

func InitFirebase() {
	opts := option.WithCredentialsJSON([]byte(config.GlobalConfig.FirebasePushConfig.ServiceAccountFileContent))
	app, err := firebase.NewApp(context.Background(), nil, opts)
	if err != nil {
		log.Logger.Fatal("error initializing firebase app", zap.Error(err))
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		log.Logger.Fatal("error initializing message client", zap.Error(err))
	}

	log.Logger.Info("init firebase message client successfully", zap.String("package_name", config.GlobalConfig.FirebasePushConfig.PackageName))
	FcmClient = client
}

func GetFcmClient() *messaging.Client {
	return FcmClient
}

func NewFirebasePushClient() *FirebasePushClient {
	if FcmClient == nil {
		InitFirebase()
	}

	return &FirebasePushClient{
		FcmClient: FcmClient,
	}
}

func (f *FirebasePushClient) Push(ctx context.Context, appId, message, token string, data map[string]string) error {
	reqMessage := MessageFirebaseItem{}
	err := jsoniter.Unmarshal([]byte(message), &reqMessage)
	if err != nil {
		log.Logger.Error("can not unmarshal the request message",
			zap.Any("message", message),
			zap.Error(err),
		)
		return err
	}

	res, err := f.FcmClient.SendAll(ctx, []*messaging.Message{
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
		return err
	}

	if res.SuccessCount <= 0 {
		log.Logger.Error("FirebasePush: no message send to firebase successfully",
			zap.Int("success_count", res.SuccessCount),
			zap.Int("failure_count", res.FailureCount),
			zap.Any("response", res.Responses),
		)
		return fmt.Errorf("FirebasePush: no message send to firebase successfully")
	}

	log.Logger.Info("FirebasePush: send message to firebase for push successfully",
		zap.Int("success_count", res.SuccessCount),
		zap.Int("failure_count", res.FailureCount),
		zap.Any("response", res.Responses))

	return nil
}
