package httpapi

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Empty is the input type for endpoints with no declared parameters.
type Empty struct{}

var inputValidator = validator.New(validator.WithRequiredStructEnabled())
var textUnmarshaler = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

type fieldSpec struct {
	index int
	source, name string
	defaultValue string
	hasDefault bool
	required bool
}

type inputSchema struct {
	fields []fieldSpec
	body map[string]fieldSpec
	hasQuery bool
}

func compileInput[I any](method, route string) (*inputSchema, error) {
	t := reflect.TypeOf((*I)(nil)).Elem()
	if t.Kind() != reflect.Struct {
		return nil, errors.New("input must be a struct")
	}
	s := &inputSchema{body: make(map[string]fieldSpec)}
	pathNames := make(map[string]bool)
	for _, part := range strings.Split(route, "/") {
		if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
			pathNames[part[1:]] = true
		}
	}
	seen := make(map[string]bool)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		spec := fieldSpec{index: i}
		for _, source := range []string{"path", "query", "header", "cookie", "json"} {
			tag, exists := f.Tag.Lookup(source)
			if !exists { continue }
			if source == "json" { tag = strings.Split(tag, ",")[0] }
			if tag == "-" { continue }
			if tag == "" || strings.TrimSpace(tag) != tag {
				return nil, fmt.Errorf("field %s: invalid %s tag", f.Name, source)
			}
			if spec.source != "" {
				return nil, fmt.Errorf("field %s: multiple input sources", f.Name)
			}
			spec.source, spec.name = source, tag
		}
		if spec.source == "" {
			if f.Tag.Get("json") == "-" && f.Tag.Get("validate") == "" {
				continue
			}
			return nil, fmt.Errorf("field %s: declare one input source or json:\"-\"", f.Name)
		}
		if f.PkgPath != "" || f.Anonymous {
			return nil, fmt.Errorf("field %s: input fields must be exported and non-embedded", f.Name)
		}
		key := spec.source + ":" + spec.name
		if spec.source == "header" { key = "header:" + strings.ToLower(spec.name) }
		if seen[key] { return nil, fmt.Errorf("field %s: duplicate input source %s", f.Name, key) }
		seen[key] = true
		spec.defaultValue, spec.hasDefault = f.Tag.Lookup("default")
		for _, rule := range strings.Split(f.Tag.Get("validate"), ",") {
			if rule == "required" { spec.required = true }
		}
		if spec.source == "path" {
			if !pathNames[spec.name] {
				return nil, fmt.Errorf("field %s: path parameter %q not found in route %q", f.Name, spec.name, route)
			}
			if spec.hasDefault { return nil, fmt.Errorf("field %s: path cannot have a default", f.Name) }
		}
		if spec.source == "json" {
			if method == http.MethodGet || method == http.MethodHead {
				return nil, errors.New("GET and HEAD cannot declare JSON body fields")
			}
			s.body[spec.name] = spec
		} else if !supportsText(f.Type, spec.source == "query" || spec.source == "header") {
			return nil, fmt.Errorf("field %s: unsupported type for %s", f.Name, spec.source)
		}
		if spec.hasDefault {
			v := reflect.New(f.Type).Elem()
			if err := applyDefault(v, spec); err != nil {
				return nil, fmt.Errorf("field %s: invalid default", f.Name)
			}
		}
		if spec.source == "query" { s.hasQuery = true }
		s.fields = append(s.fields, spec)
	}
	return s, nil
}

func supportsText(t reflect.Type, allowSlice bool) bool {
	if t.Kind() == reflect.Pointer { return supportsText(t.Elem(), allowSlice) }
	if reflect.PointerTo(t).Implements(textUnmarshaler) { return true }
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	case reflect.Slice:
		return allowSlice && supportsText(t.Elem(), false)
	default:
		return false
	}
}

func setText(v reflect.Value, values []string) error {
	if v.Kind() == reflect.Pointer {
		v.Set(reflect.New(v.Type().Elem()))
		return setText(v.Elem(), values)
	}
	if v.CanAddr() && v.Addr().Type().Implements(textUnmarshaler) {
		if len(values) != 1 { return errors.New("expected one value") }
		return v.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(values[0]))
	}
	if v.Kind() == reflect.Slice {
		s := reflect.MakeSlice(v.Type(), len(values), len(values))
		for i, value := range values {
			if err := setText(s.Index(i), []string{value}); err != nil { return err }
		}
		v.Set(s)
		return nil
	}
	if len(values) != 1 { return errors.New("expected one value") }
	raw := values[0]
	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
	case reflect.Bool:
		x, err := strconv.ParseBool(raw); if err != nil { return err }; v.SetBool(x)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x, err := strconv.ParseInt(raw, 10, v.Type().Bits()); if err != nil { return err }; v.SetInt(x)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		x, err := strconv.ParseUint(raw, 10, v.Type().Bits()); if err != nil { return err }; v.SetUint(x)
	case reflect.Float32, reflect.Float64:
		x, err := strconv.ParseFloat(raw, v.Type().Bits()); if err != nil { return err }; v.SetFloat(x)
	default:
		return errors.New("unsupported type")
	}
	return nil
}

func applyDefault(v reflect.Value, spec fieldSpec) error {
	if spec.source != "json" { return setText(v, []string{spec.defaultValue}) }
	t := v.Type()
	for t.Kind() == reflect.Pointer { t = t.Elem() }
	raw := []byte(spec.defaultValue)
	if t.Kind() == reflect.String {
		raw, _ = json.Marshal(spec.defaultValue)
	}
	return decodeField(raw, v)
}

func decodeField(raw []byte, v reflect.Value) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v.Addr().Interface()); err != nil { return err }
	var extra json.RawMessage
	if err := d.Decode(&extra); err != io.EOF { return errors.New("expected one JSON value") }
	return nil
}

func (s *inputSchema) bind(c *gin.Context, limit int64, target any) error {
	v := reflect.ValueOf(target).Elem()
	for _, param := range c.Params {
		if strings.TrimSpace(param.Value) == "" {
			return BadRequest("INVALID_PATH", "Path parameter is empty")
		}
	}
	var query url.Values
	if s.hasQuery {
		var err error
		query, err = url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil { return BadRequest("INVALID_QUERY", "Invalid query string") }
	}
	var body map[string]json.RawMessage
	if len(s.body) != 0 {
		var err error
		body, err = s.readBody(c, limit)
		if err != nil { return err }
	}
	for _, spec := range s.fields {
		f := v.Field(spec.index)
		var values []string
		var present bool
		switch spec.source {
		case "path":
			value, ok := c.Params.Get(spec.name); present = ok
			if ok { values = []string{value} }
		case "query":
			values, present = query[spec.name]
		case "header":
			values = c.Request.Header.Values(spec.name); present = len(values) != 0
			// Host is stored separately by net/http.
			if strings.EqualFold(spec.name, "Host") && c.Request.Host != "" {
				values, present = []string{c.Request.Host}, true
			}
		case "cookie":
			for _, cookie := range c.Request.Cookies() {
				if cookie.Name == spec.name { values = append(values, cookie.Value) }
			}
			present = len(values) != 0
		case "json":
			raw, ok := body[spec.name]; present = ok
			if ok {
				if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					if spec.required { return invalidField(spec) }
				} else if err := decodeField(raw, f); err != nil {
					return invalidField(spec)
				}
			}
		}
		if !present {
			if spec.hasDefault {
				if err := applyDefault(f, spec); err != nil { return invalidField(spec) }
			} else if spec.required || spec.source == "path" {
				return invalidField(spec)
			}
		} else if spec.source != "json" {
			if err := setText(f, values); err != nil { return invalidField(spec) }
		}
		if spec.required || spec.source == "path" {
			x := f
			for x.Kind() == reflect.Pointer && !x.IsNil() { x = x.Elem() }
			if x.Kind() == reflect.String && strings.TrimSpace(x.String()) == "" { return invalidField(spec) }
		}
	}
	if err := inputValidator.Struct(target); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, spec := range s.fields {
				for _, failure := range validationErrors {
					namespace := strings.Split(failure.StructNamespace(), ".")
					if len(namespace) > 1 && strings.Split(namespace[1], "[")[0] == v.Type().Field(spec.index).Name {
						return invalidField(spec)
					}
				}
			}
		}
		return BadRequest("INVALID_INPUT", "Input validation failed")
	}
	return nil
}

func invalidField(spec fieldSpec) *Error {
	code := "INVALID_" + strings.ToUpper(spec.source)
	if spec.source == "json" { code = "INVALID_BODY" }
	return BadRequest(code, fmt.Sprintf("Invalid or missing %s parameter %q", spec.source, spec.name))
}

func (s *inputSchema) readBody(c *gin.Context, limit int64) (map[string]json.RawMessage, error) {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
		return nil, NewError(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected a JSON body")
	}
	if c.Request.ContentLength > limit {
		return nil, NewError(http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "Request body is too large")
	}
	if c.Request.Body == nil { return nil, BadRequest("INVALID_JSON", "JSON body is required") }
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	d := json.NewDecoder(c.Request.Body)
	token, err := d.Token()
	if err != nil { return nil, bodyError(err) }
	if token != json.Delim('{') { return nil, bodyError(nil) }
	body := make(map[string]json.RawMessage)
	for d.More() {
		token, err := d.Token()
		if err != nil { return nil, bodyError(err) }
		key, ok := token.(string)
		if !ok { return nil, bodyError(nil) }
		if _, known := s.body[key]; !known {
			return nil, BadRequest("INVALID_JSON", "Unknown JSON field")
		}
		if _, duplicate := body[key]; duplicate {
			return nil, BadRequest("INVALID_JSON", "Duplicate JSON field")
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil { return nil, bodyError(err) }
		body[key] = raw
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') { return nil, bodyError(err) }
	var extra json.RawMessage
	if err := d.Decode(&extra); err != io.EOF { return nil, bodyError(err) }
	return body, nil
}

func bodyError(err error) *Error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return NewError(http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "Request body is too large")
	}
	return BadRequest("INVALID_JSON", "Expected exactly one valid JSON object")
}
