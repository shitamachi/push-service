package models

import (
	"context"
	"encoding/json"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/utils"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"go.uber.org/zap"
)

type BaseMessage struct {
	// 标题
	Title string `json:"title" mapstructure:"title"`
	// 内容
	Body string `json:"body" mapstructure:"body"`
	// 客户端处理通知类型
	// type: 0 无任何动作 1 打开书籍详情 2 打开一个特定的网页 3 打开新手礼包页面 4 打开奖励任务页面
	// bookId: 打开书籍详情所跳转的书籍 book id
	// link: 打开网页所跳转的网页URL
	Data map[string]string `json:"data" mapstructure:",remain"`
}

type PushMessage struct {
	// 推送消息推送的客户端名称
	appId string
	// 设备 token
	token string
	BaseMessage
}

func NewPushMessage(appId string, token string) *PushMessage {
	return &PushMessage{appId: appId, token: token}
}

func (m *PushMessage) GetAppId() string {
	return m.appId
}

func (m *PushMessage) GetToken() string {
	return m.token
}

func (m *PushMessage) SetAppId(appId string) *PushMessage {
	m.appId = appId
	return m
}

func (m *PushMessage) SetToken(token string) *PushMessage {
	m.token = token
	return m
}

func (m *PushMessage) SetTitle(title string) *PushMessage {
	m.Title = title
	return m
}

func (m *PushMessage) SetBody(body string) *PushMessage {
	m.Body = body
	return m
}

func (m *PushMessage) SetData(data map[string]string) *PushMessage {
	m.Data = data
	return m
}

func (m *PushMessage) SetBaseMessage(bs BaseMessage) *PushMessage {
	m.Body = bs.Body
	m.Title = bs.Title
	m.Data = bs.Data
	return m
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

func (m *PushMessage) Build(ctx context.Context) interface{} {
	if len(m.appId) <= 0 || len(m.token) <= 0 {
		log.WithCtx(ctx).Error("PushMessage Build app id or token not set")
		return nil
	}
	item, ok := config.GlobalConfig.ClientConfig[m.appId]
	if !ok {
		log.WithCtx(ctx).Error("PushMessage Build unknown app id")
		return nil
	}
	switch item.PushType {
	case config_entries.ApplePush:
		content := payload.NewPayload().AlertTitle(m.Title).AlertBody(m.Body)
		for k, v := range m.Data {
			content.Custom(k, v)
		}
		return &apns2.Notification{
			DeviceToken: m.token,
			Topic:       m.appId,
			Payload:     content,
		}
	case config_entries.FirebasePush:
		return &messaging.Message{
			Data: m.Data,
			Notification: &messaging.Notification{
				Title:    m.Title,
				Body:     m.Body,
				ImageURL: "",
			},
			Token: m.token,
		}
	default:
		log.WithCtx(ctx).Error("PushMessage Build \t\tlog.Logger.Error(\"PushMessage Build \")\n")
		return nil
	}
}

func (m *PushMessage) Bytes(ctx context.Context) ([]byte, error) {
	bytes, err := json.Marshal(m.ConvertToPushPayload(m.appId))
	if err != nil {
		log.WithCtx(ctx).Error("PushMessage: to bytes failed")
		return nil, err
	}
	return bytes, nil
}

func (m *PushMessage) String(ctx context.Context) (string, error) {
	bytes, err := m.Bytes(ctx)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (m *PushMessage) Clone() *PushMessage {
	return &PushMessage{
		appId: m.appId,
		token: m.token,
		BaseMessage: BaseMessage{
			Title: m.Title,
			Body:  m.Body,
			Data:  m.Data,
		},
	}
}

func (m *PushMessage) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"title": m.Title,
		"body":  m.Body,
		"data":  m.Data,
	}
}

func (m *PushMessage) ToRedisStreamValues(ctx context.Context, other map[string]interface{}) map[string]interface{} {
	return utils.MergeMap(map[string]interface{}{
		"title": m.Title,
		"body":  m.Body,
		"data":  m.BaseMessage.EncodeData(ctx),
	}, other)
}

func (bm *BaseMessage) EncodeData(ctx context.Context) string {
	bytes, err := json.Marshal(bm.Data)
	if err != nil {
		log.WithCtx(ctx).Error("EncodeData: failed to encode BaseMessage data to json")
		return ""
	}
	return string(bytes)
}

func (bm *BaseMessage) DecodeData(ctx context.Context) map[string]string {
	data, ok := bm.Data["data"]
	if !ok {
		log.WithCtx(ctx).Error("DecodeData: failed to get value from encoded BaseMessage data map[string]interface{}",
			zap.Any("message", bm),
		)
		return nil
	}
	m := make(map[string]string)
	err := json.Unmarshal([]byte(data), &m)
	if err != nil {
		log.WithCtx(ctx).Error("DecodeData: failed to decode BaseMessage data str to map[string]interface{}")
		return nil
	}
	return m
}
