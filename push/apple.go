package push

import (
	"context"
	"errors"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"go.uber.org/zap"
	"net/http"
	"reflect"
	"sync"
)

var (
	CanNotGetClientFromConfig = errors.New("can not get client from config")
)

type ApplePushClient struct {
	clients sync.Map
}

func NewApplePushClient() *ApplePushClient {
	return &ApplePushClient{}
}

func InitApplePush() {
	for bundleID, _ := range config.GlobalConfig.ApplePushConfig.Items {
		pushClientItem, err := NewApplePushClientItem(bundleID)
		if err != nil {
			log.Logger.Error("InitApplePush: can not create apple push client", zap.String("bundle_id", bundleID))
			continue
		}
		if GlobalApplePushClient == nil {
			GlobalApplePushClient = NewApplePushClient()
		}

		log.Logger.Info("InitApplePush: create apple push client successfully", zap.String("bundle_id", bundleID))
		GlobalApplePushClient.clients.Store(bundleID, pushClientItem)
	}
}

func NewApplePushClientItem(bundleID string) (*apns2.Client, error) {
	pushConfigItem, ok := config.GlobalConfig.ApplePushConfig.Items[bundleID]
	if !ok {
		return nil, CanNotGetClientFromConfig
	}
	authKey, err := token.AuthKeyFromBytes([]byte(pushConfigItem.AuthKey))
	if err != nil {
		log.Logger.Fatal("NewApplePushClient: get auth key from config failed", zap.Error(err))
		return nil, err
	}

	appleToken := &token.Token{
		AuthKey: authKey,
		// KeyID from developer account (Certificates, Identifiers & Profiles -> Keys)
		KeyID: pushConfigItem.KeyID,
		// TeamID from developer account (View Account -> Membership)
		TeamID: pushConfigItem.TeamID,
	}

	var client *apns2.Client
	switch config.GlobalConfig.Mode {
	case "debug", "test":
		client = apns2.NewTokenClient(appleToken).Development()
		log.Logger.Info("NewApplePushClient: init development apple push client successfully", zap.String("bundle_id", pushConfigItem.BundleID))
	case "release":
		client = apns2.NewTokenClient(appleToken).Production()
		log.Logger.Info("NewApplePushClient: init production apple push client successfully", zap.String("bundle_id", pushConfigItem.BundleID))
	}

	log.Logger.Info("init apple push client successfully", zap.String("bundle_id", pushConfigItem.BundleID))

	return client, nil
}

func (a *ApplePushClient) GetClientByAppID(appID string) (interface{}, bool) {
	value, ok := a.clients.Load(appID)
	if !ok {
		log.Logger.Error("GetClientByAppID: can not get apple push client from global", zap.String("bundle_id", appID))
		return nil, false
	}
	return value, true
}

func (a *ApplePushClient) Push(ctx context.Context, appId, message, token string, data map[string]string) (interface{}, error) {
	v, ok := a.GetClientByAppID(appId)
	if !ok || v == nil {
		log.Logger.Error("ApplePush: can not get push client, value is nil or get operation not ok")
		return nil, errors.New("can not get client")
	}
	client, ok := v.(*apns2.Client)
	if !ok {
		log.Logger.Error("ApplePush: got client value from global instance, but convert to *apns2.Client failed",
			zap.String("type", reflect.TypeOf(client).String()))
		return nil, errors.New("can not convert client value to *apns2.Client")
	}

	rep, err := client.Push(&apns2.Notification{
		DeviceToken: token,
		Topic:       appId,
		//example: {"aps":{"alert":"Hello!"}}
		Payload: []byte(message),
	})

	switch {
	case err != nil:
		log.Logger.Error("ApplePush: push notification failed")
		return nil, err
	case rep.StatusCode != http.StatusOK:
		log.Logger.Error("ApplePush: request send ok but apple response not ok",
			zap.Int("code", rep.StatusCode),
			zap.String("reason", rep.Reason),
		)
		return rep, fmt.Errorf("ApplePush: request send ok but apple response not ok")
	case rep.StatusCode == http.StatusOK:
		log.Logger.Info("ApplePush: request push ok")
		return rep, nil
	default:
		log.Logger.Warn("ApplePush: unknown push notification status",
			zap.Any("apple_resp", rep),
			zap.Error(err))
		return rep, fmt.Errorf("ApplePush: unknown push notification status")
	}
}
