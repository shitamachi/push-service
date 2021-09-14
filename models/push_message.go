package models

import (
	"encoding/json"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/log"
	"github.com/sideshow/apns2/payload"
	"strconv"
)

type FirebaseMessage struct {
	Data         map[string]string       `json:"data,omitempty"`
	Notification *messaging.Notification `json:"notification,omitempty"`
}
type PlatformTokenType = int

const (
	UnknownPlatform PlatformTokenType = iota
	FcmToken
	AppleDeviceToken
)

type NotificationType int

func (n *NotificationType) String() string {
	return strconv.Itoa(int(*n))
}

const (
	UnknownPushType NotificationType = iota
	OpenBook
	OpenHtmlPage
	OpenGiftPack
	OpenRewardTaskPage
)

type PushMessage struct {
	// 推送消息推送的客户端名称, 不参与 json 序列化
	appId string
	// 标题
	Title string `json:"title"`
	// 内容
	Body string `json:"body"`
	// 客户端处理通知类型 1为 firebase 2为 Apple
	Type NotificationType `json:"type"`
}

func (m *PushMessage) SetAppId(appId string) {
	m.appId = appId
}

func (m *PushMessage) ConvertToPushPayload(appId string) interface{} {
	item, ok := config.GlobalConfig.ClientConfig[appId]
	if !ok {
		return fmt.Errorf("unknown app id")
	}
	switch item.PushType {
	case config_entries.ApplePush:
		return payload.NewPayload().AlertTitle(m.Title).AlertBody(m.Body)
	case config_entries.FirebasePush:
		return &messaging.Notification{
			Title:    m.Title,
			Body:     m.Body,
			ImageURL: "",
		}
	default:
		return fmt.Errorf("unknown push type")
	}
}

func (m *PushMessage) Bytes() ([]byte, error) {
	bytes, err := json.Marshal(m.ConvertToPushPayload(m.appId))
	if err != nil {
		log.Logger.Error("PushMessage: to bytes failed")
		return nil, err
	}
	return bytes, nil
}

func (m *PushMessage) String() (string, error) {
	bytes, err := m.Bytes()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
