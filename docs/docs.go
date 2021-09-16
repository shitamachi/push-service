// GENERATED BY THE COMMAND ABOVE; DO NOT EDIT
// This file was generated by swaggo/swag

package docs

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/alecthomas/template"
	"github.com/swaggo/swag"
)

var doc = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{.Description}}",
        "title": "{{.Title}}",
        "termsOfService": "http://swagger.io/terms/",
        "contact": {
            "name": "API Support",
            "url": "http://www.swagger.io/support",
            "email": "support@swagger.io"
        },
        "license": {
            "name": "Apache 2.0",
            "url": "http://www.apache.org/licenses/LICENSE-2.0.html"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/v1/batch_push_messages": {
            "post": {
                "description": "给客户端所有用户发送push消息; 不支持每个消息单独设置",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "push"
                ],
                "summary": "给客户端所有用户发送push消息",
                "operationId": "push-messages-for-all-users",
                "parameters": [
                    {
                        "description": "请求体",
                        "name": "message",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/handler.PushMessageForAllSpecificClientReq"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "ok",
                        "schema": {
                            "allOf": [
                                {
                                    "$ref": "#/definitions/api.ResponseEntry"
                                },
                                {
                                    "type": "object",
                                    "properties": {
                                        "data": {
                                            "$ref": "#/definitions/handler.PushMessageForAllSpecificClientResp"
                                        }
                                    }
                                }
                            ]
                        }
                    },
                    "400": {
                        "description": "参数错误",
                        "schema": {
                            "$ref": "#/definitions/api.ResponseEntry"
                        }
                    },
                    "500": {
                        "description": "内部错误",
                        "schema": {
                            "$ref": "#/definitions/api.ResponseEntry"
                        }
                    }
                }
            }
        },
        "/v1/push_messages": {
            "post": {
                "description": "推送单个消息",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "push"
                ],
                "summary": "推送单个消息 push",
                "operationId": "push-messages",
                "parameters": [
                    {
                        "description": "message",
                        "name": "message",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/handler.PushMessageReqItem"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "ok",
                        "schema": {
                            "allOf": [
                                {
                                    "$ref": "#/definitions/api.ResponseEntry"
                                },
                                {
                                    "type": "object",
                                    "properties": {
                                        "data": {
                                            "type": "array",
                                            "items": {
                                                "$ref": "#/definitions/handler.PushMessageResp"
                                            }
                                        }
                                    }
                                }
                            ]
                        }
                    },
                    "400": {
                        "description": "参数错误",
                        "schema": {
                            "$ref": "#/definitions/api.ResponseEntry"
                        }
                    },
                    "404": {
                        "description": "无法从请求的 user_id 或是 token 查找到对应数据",
                        "schema": {
                            "$ref": "#/definitions/api.ResponseEntry"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/api.ResponseEntry"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "api.ResponseEntry": {
            "type": "object",
            "properties": {
                "data": {
                    "description": "实际的返回响应",
                    "type": "object"
                },
                "message": {
                    "description": "响应信息",
                    "type": "string"
                },
                "status": {
                    "description": "内部状态码",
                    "type": "integer"
                },
                "timestamp": {
                    "description": "响应时间戳",
                    "type": "integer"
                }
            }
        },
        "handler.BatchPushMessageReq": {
            "type": "object",
            "properties": {
                "global_message": {
                    "description": "全局的默认批量发送的消息, 如果具体的消息设置了 models.PushMessage 那么将覆盖掉此项",
                    "type": "object",
                    "$ref": "#/definitions/models.PushMessage"
                },
                "message_items": {
                    "description": "批量发送的消息列表",
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/handler.PushMessageReqItem"
                    }
                }
            }
        },
        "handler.BatchPushMessageRespItem": {
            "type": "object",
            "properties": {
                "app_id": {
                    "description": "app id",
                    "type": "string"
                },
                "error": {
                    "description": "错误",
                    "type": "error"
                },
                "message": {
                    "description": "推送消息",
                    "type": "object",
                    "$ref": "#/definitions/models.PushMessage"
                },
                "platform_resp": {
                    "description": "第三方平台的响应",
                    "type": "object"
                },
                "push_status": {
                    "description": "0 为 failed 1 为 succeed",
                    "type": "integer"
                },
                "reason": {
                    "description": "失败的原因",
                    "type": "string"
                },
                "token": {
                    "description": "设备 token",
                    "type": "string"
                },
                "user_id": {
                    "description": "用户 id",
                    "type": "string"
                }
            }
        },
        "handler.PushMessageForAllSpecificClientReq": {
            "type": "object",
            "properties": {
                "action_id": {
                    "description": "本次全体推送动作的唯一 id, 用于区分每次全体推送",
                    "type": "string"
                },
                "app_ids": {
                    "description": "待发送的客户端 app id",
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                },
                "message": {
                    "description": "推送消息",
                    "type": "object",
                    "$ref": "#/definitions/models.PushMessage"
                }
            }
        },
        "handler.PushMessageForAllSpecificClientResp": {
            "type": "object",
            "properties": {
                "action_id": {
                    "description": "本次推送的唯一标识符",
                    "type": "string"
                },
                "status": {
                    "description": "推送状态 1为成功",
                    "type": "integer"
                }
            }
        },
        "handler.PushMessageReqItem": {
            "type": "object",
            "properties": {
                "app_id": {
                    "description": "app id",
                    "type": "string"
                },
                "message": {
                    "description": "推送消息",
                    "type": "object",
                    "$ref": "#/definitions/models.PushMessage"
                },
                "token": {
                    "description": "设备 token",
                    "type": "string"
                },
                "user_id": {
                    "description": "用户 id",
                    "type": "string"
                }
            }
        },
        "handler.PushMessageResp": {
            "type": "object",
            "properties": {
                "error": {
                    "description": "假如请求失败, 返回的错误",
                    "type": "error"
                },
                "platform_resp": {
                    "description": "请求第三方平台发送推送消息,第三方平台返回的响应结果",
                    "type": "object"
                },
                "push_result": {
                    "description": "发布推送通知的响应信息",
                    "type": "string"
                },
                "push_status": {
                    "description": "0 为 failed 1 为 succeed",
                    "type": "integer"
                },
                "token": {
                    "description": "设备 token",
                    "type": "string"
                },
                "user_id": {
                    "description": "用户 id",
                    "type": "string"
                }
            }
        },
        "models.PushMessage": {
            "type": "object",
            "properties": {
                "body": {
                    "description": "内容",
                    "type": "string"
                },
                "data": {
                    "description": "客户端处理通知类型\ntype: 0 无任何动作 1 打开书籍详情 2 打开一个特定的网页 3 打开新手礼包页面 4 打开奖励任务页面\nbookId: 打开书籍详情所跳转的书籍 book id\nlink: 打开网页所跳转的网页URL",
                    "type": "object",
                    "additionalProperties": {
                        "type": "string"
                    }
                },
                "title": {
                    "description": "标题",
                    "type": "string"
                }
            }
        }
    }
}`

type swaggerInfo struct {
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
	Title       string
	Description string
}

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = swaggerInfo{
	Version:     "1.0",
	Host:        "petstore.swagger.io",
	BasePath:    "/v1",
	Schemes:     []string{},
	Title:       "Push Service API",
	Description: "push service http server",
}

type s struct{}

func (s *s) ReadDoc() string {
	sInfo := SwaggerInfo
	sInfo.Description = strings.Replace(sInfo.Description, "\n", "\\n", -1)

	t, err := template.New("swagger_info").Funcs(template.FuncMap{
		"marshal": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).Parse(doc)
	if err != nil {
		return doc
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, sInfo); err != nil {
		return doc
	}

	return tpl.String()
}

func init() {
	swag.Register(swag.Name, &s{})
}
