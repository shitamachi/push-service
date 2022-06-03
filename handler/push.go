package handler

import (
	"context"
	"encoding/json"
	"entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/shitamachi/push-service/api"
	"github.com/shitamachi/push-service/ent"
	"github.com/shitamachi/push-service/ent/userplatformtokens"
	"github.com/shitamachi/push-service/models"
	"github.com/shitamachi/push-service/mq"
	"github.com/shitamachi/push-service/push"
	"github.com/shitamachi/redisqueue/v2"
	"go.uber.org/zap"
	"net/http"
	"sync"
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
		c.Logger.Error("BatchPushMessageAsync: get request body failed", zap.Error(err))
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("get request body failed"))
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		c.Logger.Error("BatchPushMessageAsync: deserialize request body failed", zap.Error(err))
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("deserialize request body failed"))
	} else {
		// 如果请求传递了全局信息则为 true
		isSetGlobalMessage = !(req.GlobalMessage == nil)
	}

	if len(req.MessageItems) <= 0 {
		c.Logger.Warn("BatchPushMessageAsync: request push message items list is empty")
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
			c.Logger.Warn("BatchPushMessageAsync: one of the item in request message_items is not a valid value", zap.Any("item", reqItem))
			continue
		}

		query := c.Db.UserPlatformTokens.
			Query()
		switch {
		case len(reqItem.UserId) > 0:
			query.Where(userplatformtokens.UserID(reqItem.UserId))
		case len(reqItem.Token) > 0:
			query.Where(userplatformtokens.Token(reqItem.Token))
		default:
			c.Logger.Error("BatchPushMessageAsync: unknown query condition")
		}
		tokens, err := query.All(c.Req.Context())
		if err != nil {
			reason := fmt.Sprintf("BatchPushMessage: failed to get the user's corresponding device token, err: %v", err)
			c.Logger.Error(reason)
			continue
		}

		for _, token := range tokens {
			var message *models.PushMessage

			if isSetGlobalMessage {
				message = req.GlobalMessage.SetAppId(token.AppID).SetToken(token.Token)
			} else {
				message = reqItem.Message.SetAppId(token.AppID).SetToken(token.Token)
			}
			streamValues := message.ToRedisStreamValues(c, map[string]interface{}{
				"app_id":    token.AppID,
				"token":     token.Token,
				"user_id":   token.UserID,
				"action_id": req.ActionId,
			})
			err := c.Producer.Enqueue(&redisqueue.Message{
				Stream: mq.PushMessageStreamKey,
				Values: streamValues,
			})
			if err != nil {
				c.Logger.Error("BatchPushMessageAsync: failed to enqueue message", zap.Error(err))
				return api.Error(http.StatusInternalServerError, "failed to add push message to queue")
			}
			c.Logger.Info("BatchPushMessageAsync: add message to stream successfully")
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
		c.Logger.Error("PushMessageForAllSpecificClient: failed to get body", zap.Error(err), zap.ByteString("body", body))
		return api.ErrorWithOpts(http.StatusInternalServerError)
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		c.Logger.Error("PushMessageForAllSpecificClient: failed to unmarshal request body")
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

	ids, err := c.Db.UserPlatformTokens.Query().
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
		c.Logger.Error("PushMessageForAllSpecificClient: failed to get user_platform_token record ids records by app id list",
			zap.Strings("app_ids", req.AppIds),
			zap.Error(err),
		)
		return api.ErrorWithOpts(http.StatusInternalServerError, api.Message("failed to firstRecordOrderByUpdateAtGroupByDeviceId db"))
	}

	platformTokens := batchQueryUserPlatformTokensById(c, ids)

	for _, token := range platformTokens {
		msg := req.Message.Clone().SetToken(token.Token).SetAppId(token.AppID)
		streamValues := msg.ToRedisStreamValues(c, map[string]interface{}{
			"app_id":    token.AppID,
			"token":     token.Token,
			"user_id":   token.UserID,
			"action_id": req.ActionId,
		})
		err = c.Producer.Enqueue(&redisqueue.Message{
			Stream: mq.PushMessageStreamKey,
			Values: streamValues,
		})
		if err != nil {
			c.Logger.Error("PushMessageForAllSpecificClient: failed to enqueue message", zap.Error(err))
			return api.Error(http.StatusInternalServerError, "failed to add push message to queue")
		}

		c.Logger.Info("PushMessageForAllSpecificClient: add message to stream successfully")
	}

	return api.Ok(PushMessageForAllSpecificClientResp{Status: 1})
}

func batchQueryUserPlatformTokensById(appCtx *api.Context, ids []int) []*ent.UserPlatformTokens {
	var (
		wg                 sync.WaitGroup
		mu                 sync.Mutex
		userPlatformTokens []*ent.UserPlatformTokens
		idListLen          = len(ids)
		signalQueryCount   = 50
		lastIndex          = 0
	)

	var handlePlatformTokenQuery = func(ids ...int) {
		records, err := queryUserPlatformTokenById(appCtx, &wg, ids...)
		if err != nil {
			appCtx.Logger.Error("batchQueryUserPlatformTokensById: failed to query user_platform_token records by id list",
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

func queryUserPlatformTokenById(appCtx *api.Context, wg *sync.WaitGroup, id ...int) ([]*ent.UserPlatformTokens, error) {
	defer wg.Done()

	res, err := appCtx.Db.UserPlatformTokens.
		Query().
		Where(userplatformtokens.IDIn(id...)).
		All(context.Background())
	if err != nil {
		return nil, err
	}
	return res, nil
}
