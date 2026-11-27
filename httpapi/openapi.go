package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files/v2"
	"gopkg.in/yaml.v3"
)

// OpenAPIConfig enables documentation endpoints. Never enable them publicly
// without considering whether the API contract should be exposed.
type OpenAPIConfig struct {
	Title string
	Description string
	Version string
	SwaggerUI bool
}

type routeMetadata struct {
	summary, description, operationID string
	tags []string
	responses map[int]string
	bearer bool
	hidden bool
}

// RouteOption adds optional documentation metadata. It does not enforce
// authentication or alter handler status codes.
type RouteOption func(*routeMetadata)

func Summary(value string) RouteOption { return func(m *routeMetadata) { m.summary = value } }
func Description(value string) RouteOption { return func(m *routeMetadata) { m.description = value } }
func OperationID(value string) RouteOption { return func(m *routeMetadata) { m.operationID = value } }
func Tags(values ...string) RouteOption {
	return func(m *routeMetadata) { m.tags = append(m.tags, values...) }
}
func Response(status int, description string) RouteOption {
	if status < 200 || status > 599 { panic("httpapi: invalid documented response status") }
	return func(m *routeMetadata) { m.responses[status] = description }
}
// BearerAuth documents bearer authentication; it does not authenticate requests.
func BearerAuth() RouteOption { return func(m *routeMetadata) { m.bearer = true } }
func HideFromOpenAPI() RouteOption { return func(m *routeMetadata) { m.hidden = true } }

type object = map[string]any

type openAPIDocument struct {
	mu sync.RWMutex
	info object
	paths map[string]object
	schemas map[string]object
	securitySchemes object
}

func (s *Server) enableOpenAPI(config OpenAPIConfig) {
	if config.Title == "" { config.Title = "API" }
	if config.Version == "" { config.Version = "1.0.0" }
	s.docs = &openAPIDocument{
		info: object{"title": config.Title, "version": config.Version, "description": config.Description},
		paths: make(map[string]object),
		schemas: make(map[string]object),
		securitySchemes: make(object),
	}
	s.engine.GET("/openapi.json", func(c *gin.Context) {
		data, err := s.OpenAPIJSON()
		if err != nil { writeFailure(c, 500, "INTERNAL_ERROR", "Unable to generate OpenAPI"); return }
		c.Data(200, "application/json; charset=utf-8", data)
	})
	s.engine.GET("/openapi.yaml", func(c *gin.Context) {
		data, err := s.OpenAPIYAML()
		if err != nil { writeFailure(c, 500, "INTERNAL_ERROR", "Unable to generate OpenAPI"); return }
		c.Data(200, "application/yaml; charset=utf-8", data)
	})
	if config.SwaggerUI {
		s.engine.GET("/swagger", func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "./swagger/") })
		s.engine.GET("/swagger/*asset", func(c *gin.Context) {
			switch c.Param("asset") {
			case "/", "/index.html":
				c.Header("X-Content-Type-Options", "nosniff")
				c.Data(200, "text/html; charset=utf-8", []byte(swaggerHTML))
			case "/init.js":
				c.Data(200, "application/javascript; charset=utf-8", []byte(swaggerInit))
			case "/swagger-ui.css", "/swagger-ui-bundle.js", "/swagger-ui-standalone-preset.js":
				http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles.FS))).ServeHTTP(c.Writer, c.Request)
			default:
				writeFailure(c, 404, "NOT_FOUND", "Asset not found")
			}
		})
	}
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>API documentation</title><link rel="stylesheet" href="./swagger-ui.css"></head>
<body><div id="swagger-ui"></div><script src="./swagger-ui-bundle.js"></script>
<script src="./swagger-ui-standalone-preset.js"></script><script src="./init.js"></script></body></html>`
const swaggerInit = `window.onload = function () {
  SwaggerUIBundle({url: "../openapi.json", dom_id: "#swagger-ui",
    validatorUrl: null, persistAuthorization: false,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    layout: "StandaloneLayout"});
};`

// OpenAPIJSON returns a snapshot of all routes registered so far.
func (s *Server) OpenAPIJSON() ([]byte, error) {
	if s.docs == nil { return nil, fmt.Errorf("httpapi: OpenAPI is disabled") }
	s.docs.mu.RLock()
	defer s.docs.mu.RUnlock()
	return json.MarshalIndent(s.docs.spec(), "", "  ")
}

// OpenAPIYAML returns the same contract in YAML.
func (s *Server) OpenAPIYAML() ([]byte, error) {
	if s.docs == nil { return nil, fmt.Errorf("httpapi: OpenAPI is disabled") }
	s.docs.mu.RLock()
	defer s.docs.mu.RUnlock()
	return yaml.Marshal(s.docs.spec())
}

func (d *openAPIDocument) spec() object {
	components := object{"schemas": d.schemas}
	if len(d.securitySchemes) != 0 { components["securitySchemes"] = d.securitySchemes }
	return object{"openapi": "3.1.0", "info": d.info, "paths": d.paths, "components": components}
}

func openAPIPath(path string) (string, []string) {
	parts := strings.Split(path, "/")
	var names []string
	for i, part := range parts {
		if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
			name := part[1:]
			names = append(names, name)
			parts[i] = "{" + name + "}"
		}
	}
	return strings.Join(parts, "/"), names
}

func (d *openAPIDocument) register(method, route string, in, out reflect.Type, input *inputSchema, m routeMetadata) {
	if m.hidden { return }
	method = strings.ToLower(method)
	switch method {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
	default: return // OpenAPI does not define arbitrary HTTP operations.
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	path, pathNames := openAPIPath(route)
	op := object{}
	if m.summary != "" { op["summary"] = m.summary }
	if m.description != "" { op["description"] = m.description }
	if m.operationID != "" { op["operationId"] = m.operationID }
	if len(m.tags) != 0 { op["tags"] = append([]string(nil), m.tags...) }

	var parameters []object
	body := object{"type":"object", "additionalProperties":false}
	props := make(object)
	var requiredBody []string
	boundPath := make(map[string]bool)
	for _, f := range input.fields {
		sf := in.Field(f.index)
		schema := d.fieldSchema(sf, f.source == "json", f.source != "json")
		if f.hasDefault {
			v := reflect.New(sf.Type).Elem()
			if applyDefault(v, f) == nil {
				if raw, err := json.Marshal(v.Interface()); err == nil {
					var value any
					if json.Unmarshal(raw, &value) == nil { schema["default"] = value }
				}
			}
		}
		required := (f.required && !f.hasDefault) || f.source == "path"
		if f.source == "json" {
			if !f.required { schema = nullableSchema(schema) } else { schema = nonNullableSchema(schema) }
			props[f.name] = schema
			if required { requiredBody = append(requiredBody, f.name) }
			continue
		}
		// OpenAPI ignores these reserved header parameter names. Security
		// schemes must be declared explicitly rather than inferred as auth.
		if f.source == "header" {
			switch strings.ToLower(f.name) {
			case "authorization", "content-type", "accept": continue
			}
		}
		if f.source == "path" { boundPath[f.name] = true }
		p := object{"name":f.name, "in":f.source, "required":required, "schema":schema}
		if desc := sf.Tag.Get("description"); desc != "" { p["description"] = desc }
		if f.source == "query" && schema["type"] == "array" { p["style"] = "form"; p["explode"] = true }
		parameters = append(parameters, p)
	}
	// Even if a handler ignores a route parameter, OpenAPI requires a matching declaration.
	for _, name := range pathNames {
		if !boundPath[name] { parameters = append(parameters, object{"name":name, "in":"path", "required":true, "schema":object{"type":"string"}}) }
	}
	if len(parameters) != 0 { op["parameters"] = parameters }
	if len(props) != 0 {
		body["properties"] = props
		if len(requiredBody) != 0 { body["required"] = requiredBody }
		op["requestBody"] = object{"required":true, "content": object{"application/json":object{"schema":body}}}
	}
	if m.bearer {
		d.securitySchemes["bearerAuth"] = object{"type":"http", "scheme":"bearer"}
		op["security"] = []object{{"bearerAuth":[]string{}}}
	}

	responses := make(object)
	response := func(status int, description string) {
		if description == "" { description = http.StatusText(status) }
		if description == "" { description = "Response" }
		r := object{"description":description}
		if method != "head" && status != 204 && status != 205 && status != 304 && status >= 200 {
			t := out
			if status >= 400 { t = reflect.TypeOf(ApiResult{}) }
			r["content"] = object{"application/json":object{"schema":d.typeSchema(t, false, false)}}
		}
		responses[strconv.Itoa(status)] = r
	}
	hasSuccess := false
	for status := range m.responses { if status >= 200 && status < 300 { hasSuccess = true } }
	if !hasSuccess { response(200, "Successful response") }
	if len(input.fields) != 0 || len(pathNames) != 0 { response(400, "Invalid input") }
	if len(input.body) != 0 { response(413, "Request body too large"); response(415, "Unsupported media type") }
	response(500, "Internal server error")
	for status, description := range m.responses { response(status, description) }
	op["responses"] = responses
	if d.paths[path] == nil { d.paths[path] = make(object) }
	d.paths[path][method] = op
}
