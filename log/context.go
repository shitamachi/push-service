package log

import (
	"context"
	"go.uber.org/zap"
)

type SetLoggerToContextKey string

var key = SetLoggerToContextKey("logger")

func SetLoggerToContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, key, logger)
}

func WithCtx(ctx context.Context) *zap.Logger {
	l := ctx.Value(key).(*zap.Logger)

	if l == nil {
		l = zap.L()
	}

	return l
}
