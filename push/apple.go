package push

import (
	"context"
	"errors"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/models"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"go.uber.org/zap"
	"net/http"
	"reflect"
	"sync"
)

type ApplePushClient struct {
	clients sync.Map
}

func NewApplePushClient() *ApplePushClient {
	return &ApplePushClient{}
}

func InitApplePush(ctx context.Context) {
	for bundleID := range config.GlobalConfig.ApplePushConfig.Items {
		pushClientItem, err := NewApplePushClientItem(ctx, bundleID)
		if err != nil {
			log.WithCtx(ctx).Error("InitApplePush: can not create apple push client", zap.String("bundle_id", bundleID))
			continue
		}
		if GlobalApplePushClient == nil {
			GlobalApplePushClient = NewApplePushClient()
		}

		log.WithCtx(ctx).Info("InitApplePush: create apple push client successfully", zap.String("bundle_id", bundleID))
		GlobalApplePushClient.clients.Store(bundleID, pushClientItem)
	}
}

func NewApplePushClientItem(ctx context.Context, bundleID string) (*apns2.Client, error) {
	pushConfigItem, ok := config.GlobalConfig.ApplePushConfig.Items[bundleID]
	if !ok {
		return nil, NewWrappedError(fmt.Sprintf("init apple %s push client failed", bundleID), CanNotGetClientFromConfig)
	}
	authKey, err := token.AuthKeyFromBytes([]byte(pushConfigItem.AuthKey))
	if err != nil {
		log.WithCtx(ctx).Fatal("NewApplePushClient: get auth key from config failed", zap.Error(err))
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
		log.WithCtx(ctx).Info("NewApplePushClient: init development apple push client successfully", zap.String("bundle_id", pushConfigItem.BundleID))
	case "release":
		client = apns2.NewTokenClient(appleToken).Production()
		log.WithCtx(ctx).Info("NewApplePushClient: init production apple push client successfully", zap.String("bundle_id", pushConfigItem.BundleID))
	}

	return client, nil
}

func (a *ApplePushClient) GetClientByAppID(ctx context.Context, appID string) (interface{}, bool) {
	value, ok := a.clients.Load(appID)
	if !ok {
		log.WithCtx(ctx).Error("GetClientByAppID: can not get apple push client from global", zap.String("bundle_id", appID))
		return nil, false
	}
	return value, true
}

func (a *ApplePushClient) Push(ctx context.Context, message *models.PushMessage) (interface{}, error) {
	v, ok := a.GetClientByAppID(ctx, message.GetAppId())
	if !ok || v == nil {
		log.WithCtx(ctx).Error("ApplePush: can not get push client, value is nil or get operation not ok")
		return nil, CanNotGetPushClient
	}
	client, ok := v.(*apns2.Client)
	if !ok {
		log.WithCtx(ctx).Error("ApplePush: got client value from global instance, but convert to *apns2.Client failed",
			zap.String("type", reflect.TypeOf(client).String()))
		return nil, NewWrappedError("convert to *apns2.Client failed", ConvertToSpecificPlatformClientFailed)
	}

	notification, ok := message.Build(ctx).(*apns2.Notification)
	if !ok {
		log.WithCtx(ctx).Error("ApplePush: got message ok, but convert to *apns2.Notification failed",
			zap.String("type", reflect.TypeOf(notification).String()))
		return nil, NewWrappedError("convert to *apns2.Notification failed", ConvertToSpecificPlatformMessageFailed)
	}

	rep, err := client.PushWithContext(ctx, notification)
	if err != nil {
		log.WithCtx(ctx).Error("ApplePush: push notification failed", zap.Error(err))
		return nil, err
	}

	switch {
	case rep.StatusCode == http.StatusOK:
		log.WithCtx(ctx).Debug("ApplePush: send push message response ok", zap.Any("message", message))
		return rep, nil
	case rep.StatusCode != http.StatusOK:
		log.WithCtx(ctx).Error("ApplePush: request send ok but apple response not ok",
			zap.Int("code", rep.StatusCode),
			zap.String("reason", rep.Reason),
		)
		return rep, NewWrappedError("apple push: request send ok but apple service response not ok", SendMessageResponseNotOk)
	case errors.Is(err, context.DeadlineExceeded):
		log.WithCtx(ctx).Warn("ApplePush: send message to apns timeout")
		return nil, err
	case err != nil:
		log.WithCtx(ctx).Error("ApplePush: push notification failed", zap.Error(err))
		return nil, err
	default:
		log.WithCtx(ctx).Warn("ApplePush: unknown push notification status",
			zap.Any("apple_resp", rep),
			zap.Any("message", message),
			zap.Error(err))
		return rep, fmt.Errorf("ApplePush: unknown push notification status")
	}
}
