package httpapi_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halushko/core-go/httpapi"
)

type taskInput struct {
	Release string `path:"release" validate:"required"`
	Force bool `query:"force" default:"false"`
	Limit int `query:"limit" default:"100" validate:"min=1,max=100"`
	Labels []string `query:"label"`
	Token string `header:"Authorization" validate:"required"`
	Session string `cookie:"session" validate:"required"`
	Name string `json:"name" validate:"required"`
	Priority int `json:"priority" default:"3" validate:"min=1,max=5"`
	Enabled *bool `json:"enabled" validate:"required"`
}

func TestUnifiedInput(t *testing.T) {
	s := httpapi.New()
	calls := 0
	httpapi.POST(s, "/releases/:release/tasks", func(in *taskInput, out *result) error {
		calls++
		if in.Release != "26.1" || !in.Force || in.Limit != 100 ||
			len(in.Labels) != 2 || in.Labels[0] != "a" || in.Labels[1] != "b" ||
			in.Token != "Bearer test" || in.Session != "abc" || in.Name != "freeze" ||
			in.Priority != 3 || in.Enabled == nil || *in.Enabled {
			t.Error("incorrect input binding")
		}
		return nil
	})
	for _, tc := range []struct {
		query, body, token, cookie string
		status int
	}{
		{"force=true&label=a&label=b", `{"name":"freeze","enabled":false}`, "Bearer test", "session=abc", 200},
		{"force=true&label=a&label=b", `{"name":"freeze","enabled":false,"Release":"wrong"}`, "Bearer test", "session=abc", 400},
		{"force=true", `{"name":"freeze","enabled":false,"name":"other"}`, "Bearer test", "session=abc", 400},
		{"force=true", `{"name":"freeze","enabled":false}`, "", "session=abc", 400},
		{"force=true", `{"name":"freeze","enabled":false}`, "   ", "session=abc", 400},
		{"force=true", `{"name":"freeze","enabled":false}`, "Bearer test", "", 400},
		{"force=true", `{"name":"freeze","enabled":false}`, "Bearer test", "session=abc; session=def", 400},
		{"force=true&force=false", `{"name":"freeze","enabled":false}`, "Bearer test", "session=abc", 400},
		{"limit=", `{"name":"freeze","enabled":false}`, "Bearer test", "session=abc", 400},
		{"force=true", `{"name":"  ","enabled":false}`, "Bearer test", "session=abc", 400},
		{"force=true", `{"name":"freeze","enabled":null}`, "Bearer test", "session=abc", 400},
		{"force=true", `{"name":"freeze"}`, "Bearer test", "session=abc", 400},
		{"force=true", `{"name":"freeze","enabled":false,"priority":0}`, "Bearer test", "session=abc", 400},
	} {
		r := httptest.NewRequest("POST", "/releases/26.1/tasks?"+tc.query, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.token != "" { r.Header.Set("Authorization", tc.token) }
		if tc.cookie != "" { r.Header.Set("Cookie", tc.cookie) }
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.status { t.Errorf("expected %d, got %d: %s", tc.status, w.Code, w.Body.String()) }
	}
	if calls != 1 { t.Fatalf("unexpected handler calls: %d", calls) }
}

func TestInvalidSchema(t *testing.T) {
	type missingPath struct { ID string `path:"missing"` }
	type conflict struct { ID string `path:"id" json:"id"` }
	type invalidDefault struct { Limit int `query:"limit" default:"x"` }
	type getBody struct { Name string `json:"name"` }
	type duplicate struct { A string `header:"X-Test"`; B string `header:"x-test"` }
	type untagged struct { Name string }
	for name, register := range map[string]func(){
		"path": func(){ httpapi.GET(httpapi.New(), "/:id", func(*missingPath, *result) error { return nil }) },
		"conflict": func(){ httpapi.POST(httpapi.New(), "/:id", func(*conflict, *result) error { return nil }) },
		"default": func(){ httpapi.GET(httpapi.New(), "/", func(*invalidDefault, *result) error { return nil }) },
		"body": func(){ httpapi.GET(httpapi.New(), "/", func(*getBody, *result) error { return nil }) },
		"duplicate": func(){ httpapi.GET(httpapi.New(), "/", func(*duplicate, *result) error { return nil }) },
		"untagged": func(){ httpapi.GET(httpapi.New(), "/", func(*untagged, *result) error { return nil }) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func(){ if recover() == nil { t.Error("expected registration panic") } }()
			register()
		})
	}
}
