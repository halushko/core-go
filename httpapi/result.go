package httpapi

import (
	"fmt"
	"net/http"
)

// ApiResult is embedded in an output structure. Alerts do not change
// the HTTP status; the handler must explicitly return an error.
type ApiResult struct {
	Alerts []string `json:"alerts"`
	Code   string   `json:"code,omitempty"`
}

type ResultWithAlerts interface {
	GetAlerts() []string
	AddAlert(msg string)
}

func (r *ApiResult) GetAlerts() []string {
	if r == nil {
		return nil
	}
	return r.Alerts
}

func (r *ApiResult) AddAlert(msg string) {
	if r != nil {
		r.Alerts = append(r.Alerts, msg)
	}
}

// APIResult is promoted by embedding ApiResult and lets the wrapper
// initialize and serialize metadata without inspecting application fields.
func (r *ApiResult) APIResult() *ApiResult { return r }

type ResultPointer[T any] interface {
	*T
	APIResult() *ApiResult
}

// Error is an explicit, client-safe HTTP error. Ordinary errors are
// logged on the server and mapped to a generic 500 response.
type Error struct {
	status  int
	code    string
	message string
}

func NewError(status int, code, message string) *Error {
	return &Error{status: status, code: code, message: message}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

func (e *Error) Status() int  { return e.status }
func (e *Error) Code() string { return e.code }

func BadRequest(code, message string) *Error {
	return NewError(http.StatusBadRequest, code, message)
}

func NotFound(code, message string) *Error {
	return NewError(http.StatusNotFound, code, message)
}

func Conflict(code, message string) *Error {
	return NewError(http.StatusConflict, code, message)
}
