package handler

import (
	"context"
	"encoding/json"
	"entgo.io/ent/dialect/sql"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"github.com/google/uuid"
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
	"sync"
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
	// 异步处理推送时使用
	ActionId string `json:"action_id,omitempty"`
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

type PushMessageForAllSpecificClientReq struct {
	// 本次全体推送动作的唯一 id, 用于区分每次全体推送
	ActionId string `json:"action_id"`
	// 推送消息
	Message *models.PushMessage `json:"message,omitempty"`
	// 待发送的客户端 app id
	AppIds []string `json:"app_ids"`
}

type PushMessageForAllSpecificClientResp struct {
	// 推送状态 1为成功
	Status int `json:"status"`
	// 本次推送的唯一标识符, 如果请求没有传递则会生成一个新的标识符返回
	ActionId string `json:"action_id"`
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

	var resp PushMessageResp
	for _, token := range tokens {
		item, ok := config.GlobalConfig.ClientConfig[req.AppId]
		if !ok {
			resp = PushMessageResp{
				UserId:       token.UserID,
				Token:        token.Token,
				PushStatus:   0,
				PushResult:   "",
				PlatformResp: nil,
				Error:        errors.New("request app ID cannot match any of the items in the configuration file"),
			}
			continue
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
				resp = PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("ApplePush: can not get apple push client by app id %s", req.AppId),
				}
				break
			}
			client, ok := v.(*apns2.Client)
			if !ok {
				log.Logger.Error("ApplePush: got client value from global instance, but convert to *apns2.Client failed",
					zap.String("type", reflect.TypeOf(client).String()))
				resp = PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("inner error: get client ok but can not convert client value to *apns2.Client %s", req.AppId),
				}
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
				resp = PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("can not get firebase push client by app id %s", req.AppId),
				}
				continue
			}
			client, ok := value.(*messaging.Client)
			if !ok {
				log.Logger.Error("Push: got firebase client value from global instance, but convert to *messaging.Client failed",
					zap.String("type", reflect.TypeOf(client).String()))

				resp = PushMessageResp{
					UserId:       token.UserID,
					Token:        token.Token,
					PushStatus:   0,
					PushResult:   "",
					PlatformResp: nil,
					Error:        fmt.Errorf("inner error: can not convert client value to *messaging.Client"),
				}
				continue
			}

			// 为了获取更详细的响应结果, 使用批量发送推送接口
			res, err := client.SendAll(c.Req.Context(), []*messaging.Message{
				{
					Notification: req.Message.ConvertToPushPayload(req.AppId).(*messaging.Notification),
					Data:         req.Message.Data,
					Token:        token.Token,
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

		resp = respItem
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
				var message *models.PushMessage
				if isSetGlobalMessage {
					message = req.GlobalMessage.SetAppId(token.AppID).SetToken(token.Token)
				} else {
					message = reqItem.Message.SetAppId(token.AppID).SetToken(token.Token)
				}
				rep, err := push.GlobalApplePushClient.Push(ctx, message)

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
				var message *models.PushMessage
				if isSetGlobalMessage {
					message = req.GlobalMessage.SetAppId(token.AppID).SetToken(token.Token)
				} else {
					message = reqItem.Message.SetAppId(token.AppID).SetToken(token.Token)
				}
				rep, err := push.GlobalFirebasePushClient.Push(ctx, message)

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

// BatchPushMessageAsync godoc
// @Summary 异步批量推送消息
// @Description 异步批量推送消息, 推送结果请使用获取推送结果接口查看; 如果请求体中设置了 global_message, 那么所有消息列表中的推送消息将为 global_message, 如果具体消息里单独设置了 message 那么将会覆盖掉 global_message
// @ID push-messages-for-all-users-async
// @Tags push-async
// @Accept  json
// @Produce  json
// @Param message body BatchPushMessageReq true "请求体"
// @Success 200 {object} api.ResponseEntry{data=handler.PushMessageForAllSpecificClientResp} "ok"
// @Failure 400 {object} api.ResponseEntry "参数错误"
// @Failure 500 {object} api.ResponseEntry "内部错误"
// @Router /v1/batch_push_messages_async [post]
func BatchPushMessageAsync(c *api.Context) api.ResponseOptions {
	var req = new(BatchPushMessageReq)
	var isSetGlobalMessage bool

	body, err := c.GetBody()
	if err != nil {
		log.Logger.Error("BatchPushMessageAsync: get request body failed", zap.Error(err))
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("get request body failed"))
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Logger.Error("BatchPushMessageAsync: deserialize request body failed", zap.Error(err))
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("deserialize request body failed"))
	} else {
		// 如果请求传递了全局信息则为 true
		isSetGlobalMessage = !(req.GlobalMessage == nil)
	}

	if len(req.MessageItems) <= 0 {
		log.Logger.Warn("BatchPushMessageAsync: request push message items list is empty")
		return api.Error(http.StatusBadRequest, "request push message items list is empty")
	}

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
			log.Logger.Warn("BatchPushMessageAsync: one of the item in request message_items is not a valid value", zap.Any("item", reqItem))
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
			log.Logger.Error("BatchPushMessageAsync: unknown query condition")
		}
		tokens, err := query.All(c.Req.Context())
		if err != nil {
			reason := fmt.Sprintf("BatchPushMessage: failed to get the user's corresponding device token, err: %v", err)
			log.Logger.Error(reason)
			continue
		}

		for _, token := range tokens {
			var message *models.PushMessage
			var ctx = context.Background()

			if isSetGlobalMessage {
				message = req.GlobalMessage.SetAppId(token.AppID).SetToken(token.Token)
			} else {
				message = reqItem.Message.SetAppId(token.AppID).SetToken(token.Token)
			}
			streamValues := message.ToRedisStreamValues(map[string]interface{}{
				"app_id":    token.AppID,
				"token":     token.Token,
				"user_id":   token.UserID,
				"action_id": req.ActionId,
			})
			err = service.AddMessageToStream(
				ctx,
				service.PushMessageStreamKey,
				streamValues,
			)
			if err != nil {
				log.Logger.Error("BatchPushMessageAsync: add message to stream failed", zap.Error(err))
				return api.Error(http.StatusInternalServerError, "failed to add push message to queue")
			}
			log.Logger.Info("BatchPushMessageAsync: add message to stream successfully")
		}

	}

	// 如果请求中没有传递 action id 则会生成一个
	if len(req.ActionId) <= 0 {
		req.ActionId = uuid.NewString()
	}

	return api.Ok(PushMessageForAllSpecificClientResp{
		Status:   1,
		ActionId: req.ActionId,
	})
}

// PushMessageForAllSpecificClient godoc
// @Summary 给客户端所有用户发送push消息
// @Description 给客户端所有用户发送push消息; 不支持每条推送消息单独设置消息内容标题等信息
// @ID push-messages-for-all-users
// @Tags push
// @Accept  json
// @Produce  json
// @Param message body PushMessageForAllSpecificClientReq true "请求体"
// @Success 200 {object} api.ResponseEntry{data=PushMessageForAllSpecificClientResp} "ok"
// @Failure 400 {object} api.ResponseEntry "参数错误"
// @Failure 500 {object} api.ResponseEntry "内部错误"
// @Router /v1/batch_push_messages [post]
func PushMessageForAllSpecificClient(c *api.Context) api.ResponseOptions {
	var req = new(PushMessageForAllSpecificClientReq)
	body, err := c.GetBody()
	if err != nil {
		log.Logger.Error("PushMessageForAllSpecificClient: failed to get body", zap.Error(err), zap.ByteString("body", body))
		return api.ErrorWithOpts(http.StatusInternalServerError)
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Logger.Error("PushMessageForAllSpecificClient: failed to unmarshal request body")
		return api.ErrorWithOpts(http.StatusBadRequest, api.Message("failed to unmarshal request body"))
	}

	// we need convert []string to []interface{}
	values := make([]interface{}, len(req.AppIds))
	for i := range req.AppIds {
		values[i] = req.AppIds[i]
	}
	subQueryIdOrderByUpdateAt :=
		sql.Select(
			sql.As(sql.Distinct(userplatformtokens.FieldID), "id"),
			userplatformtokens.FieldDeviceID,
			userplatformtokens.FieldUpdatedAt,
		).
			From(sql.Table(userplatformtokens.Table)).
			Where(sql.In(userplatformtokens.FieldAppID, values...)).
			OrderBy(sql.Desc(userplatformtokens.FieldUpdatedAt)).
			As("sub_query_id_order_by_update_at")

	ids, err := db.Client.UserPlatformTokens.Query().
		Where(func(s *sql.Selector) {
			s.From(subQueryIdOrderByUpdateAt)
		}).
		Select(userplatformtokens.FieldID).
		GroupBy(userplatformtokens.FieldID).
		Ints(context.Background())
	if err != nil {
		return nil
	}

	if err != nil {
		log.Logger.Error("PushMessageForAllSpecificClient: failed to get user_platform_token record ids records by app id list",
			zap.Strings("app_ids", req.AppIds),
			zap.Error(err),
		)
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("failed to firstRecordOrderByUpdateAtGroupByDeviceId db"))
	}

	platformTokens := batchQueryUserPlatformTokensById(ids)

	for _, token := range platformTokens {
		msg := req.Message.Clone().SetToken(token.Token).SetAppId(token.AppID)
		streamValues := msg.ToRedisStreamValues(map[string]interface{}{
			"app_id":    token.AppID,
			"token":     token.Token,
			"user_id":   token.UserID,
			"action_id": req.ActionId,
		})
		err := service.AddMessageToStream(
			context.Background(),
			service.PushMessageStreamKey,
			streamValues,
		)
		if err != nil {
			log.Logger.Error("PushMessageForAllSpecificClient: add message to stream failed", zap.Error(err))
			return api.Error(http.StatusInternalServerError, "failed to add push message to queue")
		}
		log.Logger.Info("PushMessageForAllSpecificClient: add message to stream successfully")
	}

	return api.Ok(PushMessageForAllSpecificClientResp{Status: 1})
}

func appendInvalidBatchPushMessageRespItem(inValidItem []BatchPushMessageRespItem, item PushMessageReqItem, reason string) {
	inValidItem = append(inValidItem, BatchPushMessageRespItem{
		PushMessageReqItem: item,
		Reason:             reason,
	})
}

func batchQueryUserPlatformTokensById(ids []int) []*ent.UserPlatformTokens {
	var (
		wg                 sync.WaitGroup
		mu                 sync.Mutex
		userPlatformTokens []*ent.UserPlatformTokens
		idListLen          = len(ids)
		signalQueryCount   = 50
		lastIndex          = 0
	)

	var handlePlatformTokenQuery = func(ids ...int) {
		records, err := queryUserPlatformTokenById(&wg, ids...)
		if err != nil {
			log.Logger.Error("batchQueryUserPlatformTokensById: failed to query user_platform_token records by id list",
				zap.Ints("ids", ids),
				zap.Error(err),
			)
		} else {
			mu.Lock()
			userPlatformTokens = append(userPlatformTokens, records...)
			mu.Unlock()
		}
	}

	for {
		curIndex := lastIndex + signalQueryCount
		wg.Add(1)
		if idListLen >= curIndex {
			go handlePlatformTokenQuery(ids[lastIndex:curIndex]...)
			lastIndex = curIndex
		} else {
			go handlePlatformTokenQuery(ids[lastIndex:]...)
			break
		}
	}

	wg.Wait()

	return userPlatformTokens
}

func queryUserPlatformTokenById(wg *sync.WaitGroup, id ...int) ([]*ent.UserPlatformTokens, error) {
	defer wg.Done()

	res, err := db.Client.UserPlatformTokens.
		Query().
		Where(userplatformtokens.IDIn(id...)).
		All(context.Background())
	if err != nil {
		return nil, err
	}
	return res, nil
}
