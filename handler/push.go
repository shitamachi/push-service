package handler

import (
	"context"
	"encoding/json"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/db"
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/push-service/ent/userplatformtokens"
	"github.com/shitamachi/push-service/log"
	"github.com/shitamachi/push-service/models"
	"github.com/shitamachi/push-service/push"
	"github.com/shitamachi/push-service/service"
	"github.com/sideshow/apns2"
	"go.uber.org/zap"
	"net/http"
	"reflect"
	"time"
)

type PushMessageFirebaseItem push.MessageFirebaseItem

type PushMessageReqItem struct {
	// 推送消息
	Message *models.PushMessage `json:"message,omitempty"`
	// app id
	AppId string `json:"app_id"`
	// 设备 token
	Token string `json:"token"`
	// 用户 id
	UserId string `json:"user_id"`
}

type BatchPushMessageReq struct {
	// 全局的默认批量发送的消息, 如果具体的消息设置了 models.PushMessage 那么将覆盖掉此项
	GlobalMessage *models.PushMessage `json:"global_message,omitempty"`
	// 批量发送的消息列表
	MessageItems []PushMessageReqItem `json:"message_items"`
}

type BatchPushMessageRespItem struct {
	PushMessageReqItem
	// 0 为 failed 1 为 succeed
	PushStatus int `json:"push_status"`
	// 失败的原因
	Reason string `json:"reason"`
	// 第三方平台的响应
	PlatformResp interface{} `json:"platform_resp"`
	// 错误
	Error error `json:"error"`
}

type PushMessageReq struct {
	Message string   `json:"message"`
	AppId   string   `json:"app_id"`
	Tokens  []string `json:"tokens"`
	UserIds []string `json:"user_ids"`
}

type PushMessageResp struct {
	// 用户 id
	UserId string `json:"user_id"`
	// 设备 token
	Token string `json:"token"`
	// 0 为 failed 1 为 succeed
	PushStatus int `json:"push_status"`
	// 发布推送通知的响应信息
	PushResult string `json:"push_result,omitempty"`
	// 请求第三方平台发送推送消息,第三方平台返回的响应结果
	PlatformResp interface{} `json:"platform_resp,omitempty"`
	// 假如请求失败, 返回的错误
	Error error `json:"error,omitempty"`
}

// PushMessage godoc
// @Summary 推送单个消息 push
// @Description 推送单个消息
// @ID push-messages
// @Tags push
// @Accept  json
// @Produce  json
// @Param message body PushMessageReqItem true "message"
// @Success 200 {object} api.ResponseEntry{data=[]PushMessageResp} "ok"
// @Failure 400 {object} api.ResponseEntry "参数错误"
// @Failure 404 {object} api.ResponseEntry "无法从请求的 user_id 或是 token 查找到对应数据"
// @Failure 500 {object} api.ResponseEntry
// @Router /v1/push_messages [post]
func PushMessage(c *api.Context) api.ResponseOptions {
	var req PushMessageReqItem
	body, err := c.GetBody()
	if err != nil {
		return api.ErrorWithOpts(http.StatusInternalServerError)
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("can not parse request body"))
	}

	if len(req.AppId) <= 0 {
		return api.Error(http.StatusBadRequest, "app id is empty")
	} else {
		// set app id for push message
		req.Message.SetAppId(req.AppId)
	}

	// token or user_id
	if len(req.Token) <= 0 && len(req.UserId) <= 0 {
		return api.Error(http.StatusBadRequest, "can not get user_id or push token")
	}

	var tokens []*ent.UserPlatformTokens
	if len(req.UserId) > 0 {
		tokens, err = db.Client.UserPlatformTokens.
			Query().
			Where(userplatformtokens.UserID(req.UserId)).
			All(c.Req.Context())
		if err != nil && ent.IsNotFound(err) {
			return api.ErrorWithOpts(http.StatusNotFound, api.Message("can not get push token by user id"))
		} else if err != nil {
			return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("got error when find push tokens by user id"))
		}

	} else if len(req.UserId) <= 0 && len(req.Token) > 0 {
		tokens, err = db.Client.UserPlatformTokens.Query().Where(userplatformtokens.Token(req.Token)).All(context.Background())
		if err != nil && ent.IsNotFound(err) {
			return api.ErrorWithOpts(http.StatusNotFound, api.Message("not found result by request token"))
		} else if err != nil {
			return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("failed to find db record by request push token"))
		}

	}

	if len(tokens) <= 0 {
		return api.ErrorWithOpts(http.StatusNotFound, api.Message("can not found any record which will be used to push"))
	}

	var resp = make([]PushMessageResp, 0)
	for _, token := range tokens {
		item, ok := config.GlobalConfig.ClientConfig[req.AppId]
		if !ok {
			resp = append(resp, PushMessageResp{
				UserId:       token.UserID,
				Token:        token.Token,
				PushStatus:   0,
				PushResult:   "",
				PlatformResp: nil,
				Error:        errors.New("request app ID cannot match any of the items in the configuration file"),
			})
		}

		respItem := PushMessageResp{
			UserId: token.UserID,
			Token:  token.Token,
		}

		switch item.PushType {
		case "apple":
			v, ok := push.GlobalApplePushClient.GetClientByAppID(req.AppId)
			if !ok {
				log.Logger.Error("can not get apple push client by app id", zap.String("bundle_id", req.AppId))
				resp = append(resp, PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("ApplePush: can not get apple push client by app id %s", req.AppId),
				})
				break
			}
			client, ok := v.(*apns2.Client)
			if !ok {
				log.Logger.Error("ApplePush: got client value from global instance, but convert to *apns2.Client failed",
					zap.String("type", reflect.TypeOf(client).String()))
				resp = append(resp, PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("inner error: get client ok but can not convert client value to *apns2.Client %s", req.AppId),
				})
				continue
			}
			msg, _ := req.Message.Bytes()
			rep, err := client.Push(&apns2.Notification{
				DeviceToken: token.Token,
				Topic:       token.AppID,
				Expiration:  time.Now().Add(5 * time.Minute),
				//example: {"aps":{"alert":"Hello!"}}
				Payload: msg,
			})

			if err == nil && rep.StatusCode == http.StatusOK {
				respItem.PushStatus = 1
			} else {
				respItem.PushStatus = 0
			}

			respItem.Error = err
			respItem.PlatformResp = rep
		case "firebase":
			value, ok := push.GlobalFirebasePushClient.GetClientByAppID(req.AppId)
			if !ok || value == nil {
				log.Logger.Error("Firebase Push: can not get push client, value is nil or get operation not ok")
				resp = append(resp, PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("can not get firebase push client by app id %s", req.AppId),
				})
				continue
			}
			client, ok := value.(*messaging.Client)
			if !ok {
				log.Logger.Error("Push: got firebase client value from global instance, but convert to *messaging.Client failed",
					zap.String("type", reflect.TypeOf(client).String()))

				resp = append(resp, PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("inner error: can not convert client value to *messaging.Client"),
				})
				continue
			}

			// 为了获取更详细的响应结果, 使用批量发送推送接口
			res, err := client.SendAll(c.Req.Context(), []*messaging.Message{
				{
					Notification: req.Message.ConvertToPushPayload(req.AppId).(*messaging.Notification),
					Data: map[string]string{
						"type": req.Message.Type.String(),
					},
					Token: token.Token,
				},
			})
			respItem.PushStatus = 0
			respItem.PlatformResp = res
			if err != nil {
				respItem.Error = err
			} else if res.SuccessCount <= 0 || res.FailureCount >= 1 {
				if len(res.Responses) >= 1 {
					respItem.Error = res.Responses[0].Error
				}
			} else {
				respItem.PushStatus = 1
			}
		default:
			respItem.PushStatus = 0
			respItem.Error = errors.New("unknown push type")
		}

		resp = append(resp, respItem)
	}

	return api.Ok(resp)
}

// BatchPushMessage godoc
// @Summary 批量推送消息
// @Description 批量推送消息; 如果请求体中设置了 global_message, 那么所有消息列表中的推送消息将为 global_message, 如果具体消息里单独设置了 message 那么将会覆盖掉 global_message
// @ID batch-push-messages
// @Tags push
// @Accept  json
// @Produce  json
// @Param message body BatchPushMessageReq true "messages"
// @Success 200 {object} api.ResponseEntry{data=[]BatchPushMessageRespItem} "ok"
// @Failure 400 {object} api.ResponseEntry "参数错误"
// @Failure 500 {object} api.ResponseEntry "内部错误"
// @Router /v1/batch_push_messages [post]
func BatchPushMessage(c *api.Context) api.ResponseOptions {
	var req = new(BatchPushMessageReq)
	var isSetGlobalMessage bool

	body, err := c.GetBody()
	if err != nil {
		log.Logger.Error("BatchPushMessage: get request body failed", zap.Error(err))
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("get request body failed"))
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Logger.Error("BatchPushMessage: deserialize request body failed", zap.Error(err))
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("deserialize request body failed"))
	} else {
		// 如果请求传递了全局信息则为 true
		isSetGlobalMessage = !(req.GlobalMessage == nil)
	}

	if len(req.MessageItems) <= 0 {
		log.Logger.Warn("BatchPushMessage: request push message items list is empty")
		return api.Error(http.StatusBadRequest, "request push message items list is empty")
	}

	respItems := make([]BatchPushMessageRespItem, 0)

	for _, reqItem := range req.MessageItems {
		isItemValid := false
		switch {
		case len(reqItem.AppId) <= 0:
		case reqItem.Message == nil:
			if isSetGlobalMessage {
				isItemValid = true
			}
		case len(reqItem.Token) <= 0:
			if len(reqItem.UserId) > 0 {
				isItemValid = true
			}
		case len(reqItem.UserId) <= 0:
			if len(reqItem.Token) > 0 {
				isItemValid = true
			}
		default:
			isItemValid = true
		}

		if !isItemValid {
			appendInvalidBatchPushMessageRespItem(respItems, reqItem, "one of the item in request message_items is not a valid value")
			continue
		}

		query := db.Client.UserPlatformTokens.
			Query()
		switch {
		case len(reqItem.UserId) > 0:
			query.Where(userplatformtokens.UserID(reqItem.UserId))
		case len(reqItem.Token) > 0:
			query.Where(userplatformtokens.Token(reqItem.Token))
		default:
			log.Logger.Error("BatchPushMessage: unknown query condition")
		}
		tokens, err := query.All(c.Req.Context())
		if err != nil {
			reason := fmt.Sprintf("BatchPushMessage: failed to get the user's corresponding device token, err: %v", err)
			log.Logger.Error(reason)
			appendInvalidBatchPushMessageRespItem(respItems, reqItem, reason)
			continue
		}

		for _, token := range tokens {
			item, ok := config.GlobalConfig.ClientConfig[reqItem.AppId]
			if !ok {
				appendInvalidBatchPushMessageRespItem(respItems, reqItem, "can not match the app id in the request to any of the configuration files")
				break
			}

			respItem := BatchPushMessageRespItem{
				PushMessageReqItem: reqItem,
			}

			ctx := context.Background()

			switch item.PushType {
			case "apple":
				var message string
				if isSetGlobalMessage {
					req.GlobalMessage.SetAppId(token.AppID)
					message, err = req.GlobalMessage.String()
					if err != nil {
						log.Logger.Error("BatchPushMessage: convert GlobalMessage to string failed", zap.Error(err))
						respItem.Reason = "BatchPushMessage: convert GlobalMessage to string failed"
						respItem.Error = err
						continue
					}
				} else {
					message, _ = reqItem.Message.String()
				}
				rep, err := push.GlobalApplePushClient.
					Push(ctx, token.AppID, message, token.Token, nil)

				response, ok := rep.(*apns2.Response)
				if !ok {
					log.Logger.Error("apple push can not convert request response top apns2.Response type",
						zap.String("type", reflect.TypeOf(rep).String()))
					respItem.Reason = "apple push can not convert request response top apns2.Response type"
					continue
				}

				if err != nil || response.StatusCode != http.StatusOK {
					respItem.PushStatus = 0
				} else {
					respItem.PushStatus = 1
				}
				respItem.Error = err
				respItem.PlatformResp = rep

			case "firebase":
				var message string
				if isSetGlobalMessage {
					req.GlobalMessage.SetAppId(token.AppID)
					message, err = req.GlobalMessage.String()
					if err != nil {
						log.Logger.Error("BatchPushMessage: convert GlobalMessage to string failed", zap.Error(err))
						respItem.Reason = "BatchPushMessage: convert GlobalMessage to string failed"
						respItem.Error = err
						continue
					}
				} else {
					message, _ = reqItem.Message.String()
				}
				rep, err := push.GlobalFirebasePushClient.Push(ctx, token.AppID, message, token.Token, nil)

				batchResponse, ok := rep.(*messaging.BatchResponse)
				if !ok {
					log.Logger.Error("can not convert request response top firebase *BatchResponse type",
						zap.String("type", reflect.TypeOf(rep).String()))
					respItem.Reason = "can not convert request response top firebase *BatchResponse type"
					continue
				}

				respItem.PushStatus = 0
				respItem.Error = err
				if err != nil || batchResponse.SuccessCount <= 0 || !batchResponse.Responses[0].Success {
					respItem.PushStatus = 0
				} else {
					respItem.PushStatus = 1
				}
			default:
				respItem.Reason = "unknown push type"
				respItem.Error = errors.New("unknown push type")
			}

			respItems = append(respItems, respItem)
		}

	}

	return api.Ok(respItems)
}

func BatchPushMessageInQueue(c *api.Context) api.ResponseOptions {
	log.Logger.Info("execute batch push messages")

	var reqItems = make([]PushMessageReqItem, 0)
	body, err := c.GetBody()
	if err != nil {
		return api.ErrorWithOpts(http.StatusInternalServerError)
	}
	err = json.Unmarshal(body, &reqItems)
	if err != nil {
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("can not parse request body"))
	}

	if len(reqItems) <= 0 {
		return api.Error(http.StatusBadRequest, "request push item list is zero")
	}

	respItems := make([]BatchPushMessageRespItem, 0)

	ctx := context.Background()

	for _, reqItem := range reqItems {
		isItemValid := false
		switch {
		case len(reqItem.AppId) <= 0:
		//case len(reqItem.Message) <= 0:
		case len(reqItem.Token) <= 0:
			if len(reqItem.UserId) > 0 {
				isItemValid = true
			}
		case len(reqItem.UserId) <= 0:
			if len(reqItem.Token) > 0 {
				isItemValid = true
			}
		default:
			isItemValid = true
		}

		if !isItemValid {
			appendInvalidBatchPushMessageRespItem(respItems, reqItem, "one of the PushMessageReqItem field is not a valid value")
			continue
		}

		go func(reqItem PushMessageReqItem) {
			query := db.Client.UserPlatformTokens.
				Query()
			switch {
			case len(reqItem.UserId) > 0:
				query.Where(userplatformtokens.UserID(reqItem.UserId))
			case len(reqItem.Token) > 0:
				query.Where(userplatformtokens.Token(reqItem.Token))
			default:
				log.Logger.Error("BatchPushMessageInQueue: unknown query condition")
			}

			tokens, err := query.All(ctx)
			if err != nil {
				reason := fmt.Sprintf("BatchPushMessageInQueue: failed to get userplatformtokens result err: %v", err)
				log.Logger.Error(reason)
				appendInvalidBatchPushMessageRespItem(respItems, reqItem, reason)
			}

			for _, token := range tokens {
				err := service.AddMessageToStream(ctx, service.PushMessageStreamKey, map[string]interface{}{
					"message": reqItem.Message,
					"app_id":  token.AppID,
					"token":   token.Token,
					"user_id": token.UserID,
				})
				if err != nil {
					log.Logger.Error("BatchPushMessageInQueue: add message to stream failed", zap.Error(err))
					continue
				}
				log.Logger.Info("BatchPushMessageInQueue: add message to stream successfully")
			}
		}(reqItem)
	}

	return api.Ok(respItems)
}

func appendInvalidBatchPushMessageRespItem(inValidItem []BatchPushMessageRespItem, item PushMessageReqItem, reason string) {
	inValidItem = append(inValidItem, BatchPushMessageRespItem{
		PushMessageReqItem: item,
		Reason:             reason,
	})
}

func SetUserPushToken(c *api.Context) api.ResponseOptions {
	postForm := c.Req.PostForm
	if len(postForm) <= 0 {
		return api.Error(http.StatusBadRequest, "request post form is empty")
	}

	userId := postForm.Get("user_id")
	if len(userId) <= 0 {
		return api.Error(http.StatusBadRequest, "can not get user_id")
	}
	pushToken := postForm.Get("push_token")
	if len(pushToken) <= 0 {
		return api.Error(http.StatusBadRequest, "can not get push_token")
	}
	appId := postForm.Get("app_id")
	if len(pushToken) <= 0 {
		return api.Error(http.StatusBadRequest, "can not get app_id")
	}

	save, err := db.Client.UserPushToken.
		Create().
		SetUserID(userId).
		SetToken(pushToken).
		SetAppID(appId).
		Save(c.Req.Context())
	if err != nil {
		return api.Error(http.StatusInternalServerError, "can not save user token record")
	}

	return api.Ok(save)
}
