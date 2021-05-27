package handler

import "github.com/shitamachi/push-service/api"

func Health(c *api.Context) api.ResponseOptions {
	return api.Ok(nil)
}
