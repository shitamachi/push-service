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
	Message string `json:"message"`
	AppId   string `json:"app_id"`
	Token   string `json:"token"`
	UserId  string `json:"user_id"`
}

type BatchPushMessageRespItem struct {
	PushMessageReqItem
	// 0 为 failed 1 为 succeed
	PushStatus   int         `json:"push_status"`
	Reason       string      `json:"reason"`
	PlatformResp interface{} `json:"platform_resp"`
	Error        error       `json:"error"`
}

type PushMessageReq struct {
	Message string   `json:"message"`
	AppId   string   `json:"app_id"`
	Tokens  []string `json:"tokens"`
	UserIds []string `json:"user_ids"`
}

type PushMessageResp struct {
	UserId string `json:"user_id"`
	Token  string `json:"token"`
	// 0 为 failed 1 为 succeed
	PushStatus   int         `json:"push_status"`
	PushResult   string      `json:"push_result,omitempty"`
	PlatformResp interface{} `json:"platform_resp,omitempty"`
	Error        error       `json:"error,omitempty"`
}

func PushMessage(c *api.Context) api.ResponseOptions {
	fmt.Printf("")
	var req PushMessageReqItem
	body, err := c.GetBody()
	if err != nil {
		return api.ErrorWithOpts(http.StatusInternalServerError)
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("can not parse request body"))
	}

	if len(req.Message) <= 0 {
		return api.Error(http.StatusBadRequest, "can not get user_id")
	}
	if len(req.AppId) <= 0 {
		return api.Error(http.StatusBadRequest, "can not get user_id")
	}

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
			return api.ErrorWithOpts(http.StatusBadRequest, api.Message("can not get push token by user id"))
		} else if err != nil {
			return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("got error when find push tokens by user id"))
		}

	} else if len(req.UserId) <= 0 && len(req.Token) > 0 {
		tokens, err = db.Client.UserPlatformTokens.Query().Where(userplatformtokens.Token(req.Token)).All(context.Background())
		if err != nil && ent.IsNotFound(err) {
			return api.ErrorWithOpts(http.StatusBadRequest, api.Message("can not get db record by request tokens"))
		} else if err != nil {
			return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("got error when find db record by push tokens"))
		}

	}

	if len(tokens) <= 0 {
		return api.ErrorWithOpts(http.StatusBadRequest, api.Message("can not found any record which will be used to push"))
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
				Error:        errors.New("cannot match the app id in the request to any of the configuration files"),
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
					Error:        fmt.Errorf("can not get apple push client by app id %s", req.AppId),
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
					Error:        fmt.Errorf("get client ok but can not convert client value to *apns2.Client %s", req.AppId),
				})
				continue
			}
			rep, err := client.Push(&apns2.Notification{
				DeviceToken: token.Token,
				Topic:       token.AppID,
				Expiration:  time.Now().Add(5 * time.Minute),
				//example: {"aps":{"alert":"Hello!"}}
				Payload: []byte(req.Message),
			})

			if err == nil && rep.StatusCode == http.StatusOK {
				respItem.PushStatus = 1
			} else {
				respItem.PushStatus = 0
			}

			respItem.Error = err
			respItem.PlatformResp = rep
		case "firebase":
			// get push message client
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
					Error:        fmt.Errorf("can not convert client value to *messaging.Client"),
				})
				continue
			}

			res, err := client.SendAll(c.Req.Context(), []*messaging.Message{
				{
					Notification: &messaging.Notification{
						Title:    "test notification",
						Body:     "hello this is a test notification",
						ImageURL: "https://www.baidu.com/img/bd_logo1.png",
					},
					Data: map[string]string{
						"bookId": "510000751",
					},
					Token: token.Token,
				},
			})

			if err != nil {
				respItem.PushStatus = 0
				respItem.Error = err
				return api.Error(http.StatusInternalServerError, err.Error())
			} else {
				respItem.PushStatus = 1
			}
			respItem.PlatformResp = res
		default:
			respItem.PushStatus = 0
			respItem.Error = errors.New("unknown push type")
		}

		resp = append(resp, respItem)
	}

	return api.Ok(resp)
}

func BatchPushMessage(c *api.Context) api.ResponseOptions {
	fmt.Printf("execute batch push messages")
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

	for _, reqItem := range reqItems {
		isItemValid := false
		switch {
		case len(reqItem.AppId) <= 0:
		case len(reqItem.Message) <= 0:
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
			reason := fmt.Sprintf("BatchPushMessage: failed to get userplatformtokens result err: %v", err)
			log.Logger.Error(reason)
			appendInvalidBatchPushMessageRespItem(respItems, reqItem, reason)
			continue
		}

		for _, token := range tokens {
			item, ok := config.GlobalConfig.ClientConfig[reqItem.AppId]
			if !ok {
				appendInvalidBatchPushMessageRespItem(respItems, reqItem, "cannot match the app id in the request to any of the configuration files")
				break
			}

			respItem := BatchPushMessageRespItem{
				PushMessageReqItem: reqItem,
			}

			ctx := context.Background()

			switch item.PushType {
			case "apple":
				rep, err := push.GlobalApplePushClient.
					Push(ctx, token.AppID, reqItem.Message, token.Token, nil)

				response, ok := rep.(*apns2.Response)
				if !ok {
					log.Logger.Error("apple push can not convert request response top apns2.Response type",
						zap.String("type", reflect.TypeOf(rep).String()))
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
				rep, err := push.GlobalApplePushClient.Push(ctx, token.AppID, reqItem.Message, token.Token, nil)

				batchResponse, ok := rep.(*messaging.BatchResponse)
				if !ok {
					log.Logger.Error("can not convert request response top firebase *BatchResponse type",
						zap.String("type", reflect.TypeOf(rep).String()))
					continue
				}

				if err != nil || batchResponse.SuccessCount <= 0 || !batchResponse.Responses[0].Success {
					respItem.PushStatus = 1
				} else {
					respItem.PushStatus = 0
				}
				if err != nil {
					respItem.PushStatus = 0
					respItem.Error = err
					return api.Error(http.StatusInternalServerError, err.Error())
				} else {
					respItem.PushStatus = 1
				}
			default:
				respItem.PushStatus = 0
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
		case len(reqItem.Message) <= 0:
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
