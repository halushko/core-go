package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halushko/core-go/httpapi"
)

type getInput struct {
	ID string `path:"id" validate:"required"`
	Limit int `query:"limit" validate:"required,min=1"`
}

type postInput struct {
	ID string `path:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
}

type result struct {
	httpapi.ApiResult
	ID    string `json:"id"`
	Limit int    `json:"limit"`
	Name  string `json:"name"`
}

func request(s *httpapi.Server, method, path, body, contentType string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestGET(t *testing.T) {
	s := httpapi.New()
	calls := 0
	httpapi.GET(s, "/items/:id", func(in *getInput, out *result) error {
		calls++
		out.ID, out.Limit = in.ID, in.Limit
		out.AddAlert("warning")
		return nil
	})
	w := request(s, "GET", "/items/abc?limit=2", "", "")
	var out result
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || out.ID != "abc" || out.Limit != 2 || len(out.Alerts) != 1 || out.Alerts[0] != "warning" {
		t.Fatalf("unexpected response %d: %s", w.Code, w.Body.String())
	}
	for _, url := range []string{"/items/abc", "/items/abc?limit=x", "/items/abc?limit=0", "/items/%20?limit=1"} {
		w = request(s, "GET", url, "", "")
		if w.Code != 400 {
			t.Fatalf("%s: expected 400, got %d", url, w.Code)
		}
	}
	if calls != 1 {
		t.Fatalf("invalid inputs invoked handler: %d calls", calls)
	}
}

func TestPOST(t *testing.T) {
	s := httpapi.NewWithConfig(httpapi.Config{MaxBodyBytes: 40})
	calls := 0
	httpapi.POSTWithContext(s, "/items/:id", func(ctx *httpapi.Context, in *postInput, out *result) error {
		calls++
		out.ID, out.Name = in.ID, in.Name
		return ctx.SetStatus(http.StatusCreated)
	})
	w := request(s, "POST", "/items/abc", `{"name":"test"}`, "application/json")
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"alerts":[]`) || !strings.Contains(w.Body.String(), `"name":"test"`) {
		t.Fatalf("unexpected response %d: %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct {
		body, contentType string
		status            int
	}{
		{"", "application/json", 400},
		{"null", "application/json", 400},
		{`{"name":""}`, "application/json", 400},
		{`{"name":`, "application/json", 400},
		{`{"name":"x"} {}`, "application/json", 400},
		{`{"name":"x","id":"override"}`, "application/json", 400},
		{`{"name":"x"}`, "text/plain", 415},
		{`{"name":"x"}`, "", 415},
		{`{"name":"` + strings.Repeat("x", 60) + `"}`, "application/json", 413},
	} {
		w := request(s, "POST", "/items/abc", tc.body, tc.contentType)
		if w.Code != tc.status {
			t.Fatalf("body %q: expected %d, got %d: %s", tc.body, tc.status, w.Code, w.Body.String())
		}
	}
	// Simulate an unknown-length body so the streaming size check is tested.
	r := httptest.NewRequest("POST", "/items/abc", strings.NewReader(`{"name":"x"}`+strings.Repeat(" ", 60)))
	r.ContentLength = -1
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 413 || calls != 1 {
		t.Fatalf("oversized streaming body: status=%d, handler calls=%d", w.Code, calls)
	}
}

func TestErrors(t *testing.T) {
	s := httpapi.New()
	httpapi.GETWithContext(s, "/missing", func(ctx *httpapi.Context, in *httpapi.Empty, out *result) error {
		out.ID = "partial"
		out.AddAlert("warning")
		return httpapi.NotFound("ITEM_NOT_FOUND", "Item not found")
	})
	httpapi.GETWithContext(s, "/failed", func(ctx *httpapi.Context, in *httpapi.Empty, out *result) error {
		return errors.New("private database information")
	})
	httpapi.GETWithContext(s, "/panic", func(ctx *httpapi.Context, in *httpapi.Empty, out *result) error {
		panic("secret panic")
	})
	httpapi.GETWithContext(s, "/empty", func(ctx *httpapi.Context, in *httpapi.Empty, out *result) error {
		return ctx.SetStatus(http.StatusNoContent)
	})
	type broken struct {
		httpapi.ApiResult
		Channel chan int `json:"channel"`
	}
	httpapi.GETWithContext(s, "/broken", func(ctx *httpapi.Context, in *httpapi.Empty, out *broken) error {
		return nil
	})
	for _, tc := range []struct {
		method, path, code string
		status             int
	}{
		{"GET", "/missing", "ITEM_NOT_FOUND", 404},
		{"GET", "/failed", "INTERNAL_ERROR", 500},
		{"GET", "/panic", "INTERNAL_ERROR", 500},
		{"GET", "/broken", "INTERNAL_ERROR", 500},
		{"GET", "/unknown", "NOT_FOUND", 404},
		{"POST", "/missing", "METHOD_NOT_ALLOWED", 405},
		{"GET", "/empty", "", 204},
	} {
		w := request(s, tc.method, tc.path, "", "")
		if w.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d: %s", tc.path, tc.status, w.Code, w.Body.String())
		}
		if tc.status == 204 {
			if w.Body.Len() != 0 {
				t.Fatal("204 must not have a body")
			}
			continue
		}
		var meta httpapi.ApiResult
		if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil || meta.Code != tc.code {
			t.Fatalf("%s: wrong metadata %s", tc.path, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private database") || strings.Contains(w.Body.String(), "secret panic") || strings.Contains(w.Body.String(), "partial") {
			t.Fatal("leaked internal or partial data")
		}
		if tc.path == "/missing" && tc.method == "GET" && len(meta.Alerts) != 2 {
			t.Fatalf("alerts lost: %s", w.Body.String())
		}
	}
}
