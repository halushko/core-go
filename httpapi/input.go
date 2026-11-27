package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// Empty marks an unused input source.
type Empty struct{}

// Input keeps different input sources isolated. A JSON field cannot
// override a path or query parameter.
type Input[P, Q, B any] struct {
	Path  P
	Query Q
	Body  B
}

func isEmpty[T any]() bool {
	var zero T
	_, ok := any(zero).(Empty)
	return ok
}

func bindInput[P, Q, B any](c *gin.Context, limit int64, in *Input[P, Q, B]) error {
	for _, param := range c.Params {
		if strings.TrimSpace(param.Value) == "" {
			return BadRequest("INVALID_PATH", "Path parameter is empty")
		}
	}
	if !isEmpty[P]() {
		if err := c.ShouldBindUri(&in.Path); err != nil {
			return BadRequest("INVALID_PATH", "Invalid or missing path parameter")
		}
	}
	if !isEmpty[Q]() {
		// Unlike URL.Query(), ParseQuery reports malformed escape sequences.
		if _, err := url.ParseQuery(c.Request.URL.RawQuery); err != nil {
			return BadRequest("INVALID_QUERY", "Invalid query string")
		}
		if err := c.ShouldBindQuery(&in.Query); err != nil {
			return BadRequest("INVALID_QUERY", "Invalid or missing query parameter")
		}
	}
	if !isEmpty[B]() {
		return bindBody(c, limit, &in.Body)
	}
	return nil
}

func bindBody[B any](c *gin.Context, limit int64, target *B) error {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
		return NewError(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected a JSON body")
	}
	if c.Request.ContentLength > limit {
		return NewError(http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "Request body is too large")
	}

	if c.Request.Body == nil {
		return BadRequest("INVALID_JSON", "JSON body is required")
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var body *B
	if err := decoder.Decode(&body); err != nil {
		return bodyError(err)
	}
	if body == nil {
		return BadRequest("INVALID_JSON", "JSON body must not be null")
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return bodyError(err)
	}
	if err := binding.Validator.ValidateStruct(body); err != nil {
		return BadRequest("INVALID_BODY", "Invalid or missing body field")
	}
	*target = *body
	return nil
}

func bodyError(err error) *Error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return NewError(http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "Request body is too large")
	}
	return BadRequest("INVALID_JSON", "Expected exactly one valid JSON value")
}
