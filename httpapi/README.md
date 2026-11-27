# HTTP API wrapper

A small typed HTTP server built on Gin, which remains an implementation detail.
No Gin imports, JSON serialization or manual response sending are needed in
application code.

```go
import (
    "net/http"
    "github.com/halushko/core-go/httpapi"
)

type ItemPath struct {
    ID string `uri:"id" binding:"required"`
}
type ItemQuery struct {
    Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
}
type ItemBody struct {
    Name string `json:"name" binding:"required"`
}
type ItemResult struct {
    httpapi.ApiResult
    ID string `json:"id"`
    Name string `json:"name"`
}

func register(server *httpapi.Server) {
    httpapi.GET(server, "/items/:id",
        func(ctx *httpapi.Context, in httpapi.Input[ItemPath, ItemQuery, httpapi.Empty], out *ItemResult) error {
            out.ID = in.Path.ID
            out.Name = "Example"
            out.AddAlert("Optional warning")
            return nil
        })

    httpapi.POST(server, "/items",
        func(ctx *httpapi.Context, in httpapi.Input[httpapi.Empty, httpapi.Empty, ItemBody], out *ItemResult) error {
            out.ID = "new-id"
            out.Name = in.Body.Name
            return ctx.SetStatus(http.StatusCreated)
        })
}
```

Start with `server := httpapi.New()`, register routes, then call
`server.Run(":8080")`, or use it as a standard `http.Handler` with your own
`http.Server`. Use `ctx.Context()` for cancellation and deadlines in
downstream calls.

The output embeds `ApiResult` **by value**. The wrapper allocates it per
request. Successful output is flat JSON:
`{"alerts":["Optional warning"],"id":"123","name":"Example"}`.
Empty alerts are serialized as an array, not null. `Code` is an optional
machine-readable application code, independent of the HTTP status.
`ResultWithAlerts` matches the GetAlerts/AddAlert pattern for reusable
business helpers; replace local duplicate definitions with aliases where useful.

Input sources are independent: `uri` binds path values, `form` binds query
values, and `json` binds JSON. Use Gin-compatible `binding` validation tags
(e.g. required, min, max, oneof). The `Empty` marker disables an input source.
Required missing query/body fields fail before the handler runs. Whitespace-only
route parameters are rejected; an absent path segment that does not match the
route returns 404. Use domain-specific validation for whitespace-only body or
query strings where needed. GET/HEAD do not bind JSON bodies.

For failures, return a client-safe error such as
`httpapi.NotFound("ITEM_NOT_FOUND", "Item not found")`,
`httpapi.BadRequest("INVALID_ITEM", "Invalid item")`, or
`httpapi.NewError(503, "UPSTREAM_UNAVAILABLE", "Service unavailable")`.
Ordinary Go errors are logged and mapped to a generic 500. Failed requests
return only metadata, retaining any alerts already collected:
`{"alerts":["Item not found"],"code":"ITEM_NOT_FOUND"}`.
Warnings alone keep the successful status (200 by default). Explicit success
statuses use `ctx.SetStatus`; 204/205 and HEAD responses have no body.

JSON requests require a JSON Content-Type, reject unknown fields, null and
multiple JSON values, and default to a 1 MiB body limit
(`NewWithConfig(Config{MaxBodyBytes: ...})` overrides it).
Malformed input returns 400, oversized bodies 413, unsupported media types
415, unknown routes 404, and unsupported methods 405. Handler panics and
output serialization failures return a generic 500.

This first version focuses on JSON endpoints. Authentication, CORS, multipart,
streaming, raw response writers and OpenAPI generation are deliberately not
part of the initial API.

## Tests

```sh
cd httpapi
go mod tidy
go test -race ./...
```
