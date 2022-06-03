package log

import (
	"context"
	"github.com/shitamachi/push-service/api"
	"go.uber.org/zap"
)

type SetLoggerToContextKey string

var key = SetLoggerToContextKey("logger")

func SetLoggerToContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, key, logger)
}

func WithCtx(ctx context.Context) *zap.Logger {
	var l *zap.Logger
	switch ctx.(type) {
	case *api.Context:
		l = ctx.(*api.Context).Logger
	default:
		v := ctx.Value(key)
		l = v.(*zap.Logger)
	}

	if l == nil {
		l = zap.L()
	}

	return l
}
