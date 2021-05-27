package push

import (
	"context"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"go.uber.org/zap"
	"net/http"
	"sync"
)

type ApplePushClient struct {
	Client *apns2.Client
}

var applePushClient *apns2.Client
var rwl sync.RWMutex

func InitApplePush() {
	authKey, err := token.AuthKeyFromBytes([]byte(config.GlobalConfig.ApplePushConfig.AuthKey))
	if err != nil {
		log.Logger.Fatal("get auth key from config failed", zap.Error(err))
		return
	}

	appleToken := &token.Token{
		AuthKey: authKey,
		// KeyID from developer account (Certificates, Identifiers & Profiles -> Keys)
		KeyID: config.GlobalConfig.ApplePushConfig.KeyID,
		// TeamID from developer account (View Account -> Membership)
		TeamID: config.GlobalConfig.ApplePushConfig.TeamID,
	}

	var client *apns2.Client
	switch config.GlobalConfig.Mode {
	case "debug", "test":
		client = apns2.NewTokenClient(appleToken).Development()
		log.Logger.Info("InitApplePush: init development apple push client successfully")
	case "release":
		client = apns2.NewTokenClient(appleToken).Production()
		log.Logger.Info("InitApplePush: init production apple push client successfully")
	}

	log.Logger.Info("init apple push client successfully", zap.String("bundle_id", config.GlobalConfig.ApplePushConfig.BundleID))
	applePushClient = client
}

func GetApplePushClient() *apns2.Client {
	rwl.RLock()
	defer rwl.RUnlock()
	return applePushClient
}

func NewApplePushClient() *ApplePushClient {
	if GetApplePushClient() == nil {
		InitApplePush()
	}

	return &ApplePushClient{
		Client: GetApplePushClient(),
	}
}

func (a *ApplePushClient) Push(ctx context.Context, appId, message, token string, data map[string]string) error {
	client := GetApplePushClient()
	if client == nil {
		InitApplePush()
	}

	rep, err := a.Client.Push(&apns2.Notification{
		DeviceToken: token,
		Topic:       appId,
		//example: {"aps":{"alert":"Hello!"}}
		Payload: []byte(message),
	})

	switch {
	case err != nil:
		log.Logger.Error("ApplePush: push notification failed")
		return err
	case rep.StatusCode != http.StatusOK:
		log.Logger.Error("ApplePush: request send ok but apple response not ok",
			zap.Int("code", rep.StatusCode),
			zap.String("reason", rep.Reason),
		)
		return fmt.Errorf("ApplePush: request send ok but apple response not ok")
	case rep.StatusCode == http.StatusOK:
		log.Logger.Info("ApplePush: request push ok")
		return nil
	default:
		log.Logger.Warn("ApplePush: unknown push notification status",
			zap.Any("apple_resp", rep),
			zap.Error(err))
		return fmt.Errorf("ApplePush: unknown push notification status")
	}
}
