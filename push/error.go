package push

import (
	"errors"
	"fmt"
)

var (
	CanNotGetClientFromConfig              = errors.New("can not get client from config")
	CanNotGetPushClient                    = errors.New("can not get push client")
	ConvertToSpecificPlatformClientFailed  = errors.New("convert to specific platform client failed")
	ConvertToSpecificPlatformMessageFailed = errors.New("convert to specific platform message failed")
	SendMessageResponseNotOk               = errors.New("request send message to platform push service reply http status code not ok")
)

type WrappedError struct {
	msg string
	err error
}

func NewWrappedError(msg string, err error) *WrappedError {
	return &WrappedError{msg: msg, err: err}
}

func (w *WrappedError) Error() string {
	return fmt.Sprintf("message=%s, extra message=%s", w.err.Error(), w.msg)
}

func (w *WrappedError) Unwrap() error {
	return w.err
}
