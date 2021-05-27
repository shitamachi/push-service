package api

import (
	"encoding/json"
	"io"
	"net/http"
)

type Context struct {
	Writer http.ResponseWriter
	Req    *http.Request
}

type ResponseData = interface{}

type ResponseEntry struct {
	Status    int          `json:"status"`
	Message   string       `json:"message"`
	Data      ResponseData `json:"data,omitempty"`
	Timestamp int64        `json:"timestamp"`
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

func (c *Context) GetBody() ([]byte, error) {
	return io.ReadAll(c.Req.Body)
}
