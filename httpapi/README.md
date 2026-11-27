# HTTP API wrapper

Typed JSON endpoints. Gin is private to the module; handlers receive their
own application input and output structures.

```go
import "github.com/halushko/core-go/httpapi"

type TaskInput struct {
    Release  string `path:"release" validate:"required"`
    Force    bool   `query:"force" default:"false"`
    Token    string `header:"Authorization" validate:"required"`
    Session  string `cookie:"session"`
    Name     string `json:"name" validate:"required"`
    Priority int    `json:"priority" default:"3" validate:"min=1,max=5"`
}

type TaskResult struct {
    httpapi.ApiResult
    ID string `json:"id"`
}

func register(server *httpapi.Server) {
    httpapi.POST(server, "/api/releases/:release/tasks",
        func(in *TaskInput, out *TaskResult) error {
            out.ID = "new-id"
            out.AddAlert("Optional warning")
            return nil
        })
}
```

The wrapper allocates both structures per request, binds input, validates it,
runs the handler and sends the output. Embed ApiResult by value in the output.
Alerts are a flat JSON array; Code is an optional application code.
Use Empty for routes without input. Use New(), then Run(":8080") or use the
server as a standard http.Handler.

## Input tags

| Tag | Source |
| --- | --- |
| path | Route parameter, e.g. :release |
| query | URL query parameter |
| header | HTTP header (case insensitive) |
| cookie | HTTP cookie |
| json | JSON body object field |
| validate | go-playground/validator rules |
| default | Value used only when the field is absent |

Declare exactly one source per exported top-level input field; json:"-" skips
a field without a source. Nested JSON objects are supported using ordinary
JSON structs and validate tags. Embedded input fields are not supported.
Source names are independent; JSON cannot overwrite path, header, cookie or
query fields. Unknown and duplicate top-level JSON keys are rejected; nested
JSON follows encoding/json semantics with unknown-field rejection for structs.

Text sources support strings, bools, integer/float types, pointers,
encoding.TextUnmarshaler and slices for repeated query/header values.
Scalar duplicate values are rejected. Defaults use text for text sources
and JSON literals for body fields, except JSON strings use plain text.
Defaults do not replace explicit zero, false, empty strings or null.

Validation runs before the handler. Required strings reject whitespace.
The standard validator required rule also rejects zero and false on
non-pointer fields: use *bool or *int with required to distinguish an
explicit false/zero from a missing value. Optional null JSON leaves the
field zero-valued; required null fails. JSON input requires an object even
when all body fields are optional. Header/cookie binding is not authentication.

Invalid route schema panics at registration with a diagnostic: unknown path
parameter, conflicting sources, duplicate bindings, invalid default type,
unsupported field types or JSON body fields on GET/HEAD. Validation rules
are evaluated by the validator on requests.

## Context and errors

GET, POST, PUT, PATCH, DELETE, HEAD and Handle use func(*Input, *Output) error.
Each has a WithContext variant for cancellation, extra headers and custom
success status:

```go
httpapi.POSTWithContext(server, "/api/releases/:release/tasks",
    func(ctx *httpapi.Context, in *TaskInput, out *TaskResult) error {
        out.ID = "new-id"
        return ctx.SetStatus(201)
    })
```

Return httpapi.NotFound("ITEM_NOT_FOUND", "Item not found"),
httpapi.BadRequest(...), httpapi.Conflict(...) or httpapi.NewError(status, code,
message) for client-safe errors. Other Go errors are logged and return a
generic 500. Errors return metadata only, preserving previously added alerts.
Warnings alone do not change HTTP status. Default success is 200; 204/205
and HEAD have no body.

JSON endpoints require a JSON Content-Type and enforce a configurable 1 MiB
default body limit. Malformed input is 400, oversized bodies 413, unsupported
media types 415, missing routes 404, unsupported methods 405, panics and
encoding failures 500. Authentication middleware, CORS, streaming and
OpenAPI are outside this first version.

## Tests

```sh
cd httpapi
go mod tidy
go test -race ./...
```
