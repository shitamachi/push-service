package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"time"
)

type Context struct {
	*AppContext
	Writer http.ResponseWriter
	Req    *http.Request
}

type ResponseData interface{}

type ResponseEntry struct {
	// 内部状态码
	Status int `json:"status"`
	// 响应信息
	Message string `json:"message"`
	// 实际的返回响应
	Data interface{} `json:"data,omitempty"`
	// 响应时间戳
	Timestamp int64 `json:"timestamp"`
}

type Response struct {
	HttpCode int `json:"-"`
	ResponseEntry
}

type ResponseOptions = []ResponseOption

type ResponseOption interface {
	apply(r *Response)
}

type optionFunc func(r *Response)

func (f optionFunc) apply(r *Response) {
	f(r)
}

func HttpCode(code int) ResponseOption {
	return optionFunc(func(r *Response) {
		r.HttpCode = code
	})
}

func Status(status int) ResponseOption {
	return optionFunc(func(r *Response) {
		r.Status = status
	})
}

func Message(message string) ResponseOption {
	return optionFunc(func(r *Response) {
		r.Message = message
	})
}

func Data(data ResponseData) ResponseOption {
	return optionFunc(func(r *Response) {
		r.Data = data
	})
}

func HandleFunc(pattern string, f func(ctx *Context) []ResponseOption) {
	http.DefaultServeMux.HandleFunc(pattern, func(writer http.ResponseWriter, request *http.Request) {
		responseOptions := f(&Context{
			Writer: writer,
			Req:    request,
		})
		response := NewResponse(responseOptions)

		bytes, err := json.Marshal(response)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		writer.WriteHeader(response.HttpCode)
		//goland:noinspection ALL
		writer.Write(bytes)
	})
}

func (ctx *Context) WrapperGinHandleFunc(f func(ctx *Context) ResponseOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		responseOptions := f(&Context{
			AppContext: ctx.AppContext,
			Writer:     c.Writer,
			Req:        c.Request,
		})
		response := NewResponse(responseOptions)

		bytes, err := json.Marshal(response)
		if err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		c.Writer.WriteHeader(response.HttpCode)
		//goland:noinspection ALL
		c.Writer.Write(bytes)
	}
}

func NewResponse(opts []ResponseOption) *Response {
	var r Response
	for _, opt := range opts {
		opt.apply(&r)
	}

	return &r
}

func New(code, status int, message string, data ResponseData) []ResponseOption {
	return []ResponseOption{
		HttpCode(code),
		Status(status),
		Message(message),
		Data(data),
	}
}

func Ok(data ResponseData) []ResponseOption {
	return []ResponseOption{
		HttpCode(http.StatusOK),
		Status(http.StatusOK),
		Message("ok"),
		Data(data),
	}
}

func Error(code int, message string) []ResponseOption {
	return []ResponseOption{
		HttpCode(code),
		Status(code),
		Message(message),
	}
}

func ErrorWithOpts(httpCode int, opts ...ResponseOption) []ResponseOption {
	return append(opts, HttpCode(httpCode))
}

func (ctx *Context) GetBody() ([]byte, error) {
	return io.ReadAll(ctx.Req.Body)
}

func (ctx *Context) Deadline() (deadline time.Time, ok bool) {
	if ctx.Req == nil || ctx.Req.Context() == nil {
		return
	}
	return ctx.Req.Context().Deadline()
}

func (ctx *Context) Done() <-chan struct{} {
	if ctx.Req == nil || ctx.Req.Context() == nil {
		return nil
	}
	return ctx.Req.Context().Done()
}

func (ctx *Context) Err() error {
	if ctx.Req == nil || ctx.Req.Context() == nil {
		return nil
	}
	return ctx.Req.Context().Err()
}

func (ctx *Context) Value(key any) any {
	v := ctx.AppContext.Value(key)
	if v != nil {
		return v
	}
	if ctx.Req == nil || ctx.Req.Context() == nil {
		return nil
	}
	return ctx.Req.Context().Value(key)
}
