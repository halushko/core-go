package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type Handler[I, O any] func(*I, *O) error

type ContextHandler[I, O any] func(*Context, *I, *O) error

// Handle binds and validates inputs, invokes the typed handler once,
// and sends the output automatically. O must embed ApiResult by value.
func HandleWithContext[I, O any, R ResultPointer[O]](s *Server, method, path string, handler ContextHandler[I, O]) {
	if handler == nil {
		panic("httpapi: nil handler")
	}
	schema, err := compileInput[I](method, path)
	if err != nil {
		panic(fmt.Sprintf("httpapi: %s %s: %v", method, path, err))
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

		var in I
		ctx := &Context{ctx: c.Request.Context(), header: c.Request.Header, status: http.StatusOK}
		err := schema.bind(c, s.maxBodyBytes, &in)
		if err == nil {
			err = handler(ctx, &in, out)
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

// Handle registers a handler that needs no request context.
func Handle[I, O any, R ResultPointer[O]](s *Server, method, path string, handler Handler[I, O]) {
	if handler == nil { panic("httpapi: nil handler") }
	HandleWithContext[I, O, R](s, method, path, func(_ *Context, in *I, out *O) error {
		return handler(in, out)
	})
}

func GET[I, O any, R ResultPointer[O]](s *Server, path string, handler Handler[I, O]) {
	Handle[I, O, R](s, http.MethodGet, path, handler)
}

func GETWithContext[I, O any, R ResultPointer[O]](s *Server, path string, handler ContextHandler[I, O]) {
	HandleWithContext[I, O, R](s, http.MethodGet, path, handler)
}

func POST[I, O any, R ResultPointer[O]](s *Server, path string, handler Handler[I, O]) {
	Handle[I, O, R](s, http.MethodPost, path, handler)
}

func POSTWithContext[I, O any, R ResultPointer[O]](s *Server, path string, handler ContextHandler[I, O]) {
	HandleWithContext[I, O, R](s, http.MethodPost, path, handler)
}

func PUT[I, O any, R ResultPointer[O]](s *Server, path string, handler Handler[I, O]) {
	Handle[I, O, R](s, http.MethodPut, path, handler)
}

func PUTWithContext[I, O any, R ResultPointer[O]](s *Server, path string, handler ContextHandler[I, O]) {
	HandleWithContext[I, O, R](s, http.MethodPut, path, handler)
}

func PATCH[I, O any, R ResultPointer[O]](s *Server, path string, handler Handler[I, O]) {
	Handle[I, O, R](s, http.MethodPatch, path, handler)
}

func PATCHWithContext[I, O any, R ResultPointer[O]](s *Server, path string, handler ContextHandler[I, O]) {
	HandleWithContext[I, O, R](s, http.MethodPatch, path, handler)
}

func DELETE[I, O any, R ResultPointer[O]](s *Server, path string, handler Handler[I, O]) {
	Handle[I, O, R](s, http.MethodDelete, path, handler)
}

func DELETEWithContext[I, O any, R ResultPointer[O]](s *Server, path string, handler ContextHandler[I, O]) {
	HandleWithContext[I, O, R](s, http.MethodDelete, path, handler)
}

func HEAD[I, O any, R ResultPointer[O]](s *Server, path string, handler Handler[I, O]) {
	Handle[I, O, R](s, http.MethodHead, path, handler)
}

func HEADWithContext[I, O any, R ResultPointer[O]](s *Server, path string, handler ContextHandler[I, O]) {
	HandleWithContext[I, O, R](s, http.MethodHead, path, handler)
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
