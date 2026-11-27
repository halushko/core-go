package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Context exposes request-scoped features without exposing Gin.
type Context struct {
	ctx    context.Context
	header http.Header
	status int
}

func (c *Context) Context() context.Context { return c.ctx }

func (c *Context) Header(name string) string { return c.header.Get(name) }

// SetStatus sets an explicit successful HTTP status. Return errors
// from a handler to send non-2xx statuses.
func (c *Context) SetStatus(status int) error {
	if status < 200 || status >= 300 {
		return errors.New("success status must be 2xx")
	}
	c.status = status
	return nil
}

type Handler[P, Q, B, O any] func(*Context, Input[P, Q, B], *O) error

// Handle binds and validates inputs, invokes the typed handler once,
// and sends the output automatically. O must embed ApiResult by value.
func Handle[P, Q, B, O any, R ResultPointer[O]](s *Server, method, path string, handler Handler[P, Q, B, O]) {
	if handler == nil {
		panic("httpapi: nil handler")
	}
	if (method == http.MethodGet || method == http.MethodHead) && !isEmpty[B]() {
		panic("httpapi: GET and HEAD cannot declare a JSON body")
	}
	s.engine.Handle(method, path, func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("httpapi: handler panic", "route", c.FullPath())
				if !c.Writer.Written() {
					writeFailure(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
				}
			}
		}()

		out := R(new(O))
		meta := out.APIResult()
		if meta == nil {
			writeFailure(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			return
		}
		meta.Alerts = []string{}

		var in Input[P, Q, B]
		ctx := &Context{ctx: c.Request.Context(), header: c.Request.Header, status: http.StatusOK}
		err := bindInput(c, s.maxBodyBytes, &in)
		if err == nil {
			err = handler(ctx, in, out)
		}
		if err != nil {
			status, code, message := classifyError(err, c.FullPath())
			// Preserve alerts, but do not expose partially populated business
			// data after a failed request.
			meta.Code = code
			meta.AddAlert(message)
			writeJSON(c, status, meta)
			return
		}
		if meta.Alerts == nil {
			meta.Alerts = []string{}
		}
		writeJSON(c, ctx.status, out)
	})
}

func GET[P, Q, O any, R ResultPointer[O]](s *Server, path string, handler Handler[P, Q, Empty, O]) {
	Handle[P, Q, Empty, O, R](s, http.MethodGet, path, handler)
}

func POST[P, Q, B, O any, R ResultPointer[O]](s *Server, path string, handler Handler[P, Q, B, O]) {
	Handle[P, Q, B, O, R](s, http.MethodPost, path, handler)
}

func PUT[P, Q, B, O any, R ResultPointer[O]](s *Server, path string, handler Handler[P, Q, B, O]) {
	Handle[P, Q, B, O, R](s, http.MethodPut, path, handler)
}

func PATCH[P, Q, B, O any, R ResultPointer[O]](s *Server, path string, handler Handler[P, Q, B, O]) {
	Handle[P, Q, B, O, R](s, http.MethodPatch, path, handler)
}

func DELETE[P, Q, B, O any, R ResultPointer[O]](s *Server, path string, handler Handler[P, Q, B, O]) {
	Handle[P, Q, B, O, R](s, http.MethodDelete, path, handler)
}

func classifyError(err error, route string) (int, string, string) {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr != nil && apiErr.status >= 400 && apiErr.status <= 599 && apiErr.code != "" && apiErr.message != "" {
		return apiErr.status, apiErr.code, apiErr.message
	}
	slog.Error("httpapi: handler failed", "route", route, "error", err)
	return http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error"
}

func writeFailure(c *gin.Context, status int, code, message string) {
	writeJSON(c, status, &ApiResult{Code: code, Alerts: []string{message}})
}

func writeJSON(c *gin.Context, status int, value any) {
	if status == http.StatusNoContent || status == http.StatusResetContent || c.Request.Method == http.MethodHead {
		c.Status(status)
		return
	}
	// Marshal before writing headers so serialization failures become a 500
	// rather than a truncated successful response.
	data, err := json.Marshal(value)
	if err != nil {
		slog.Error("httpapi: response encoding failed", "route", c.FullPath(), "error", err)
		c.Data(http.StatusInternalServerError, "application/json; charset=utf-8", []byte(`{"alerts":["Internal server error"],"code":"INTERNAL_ERROR"}`))
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}
