package httpapi_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halushko/core-go/httpapi"
	"gopkg.in/yaml.v3"
)

type docInput struct {
	Release string `path:"release" validate:"required" description:"Release version" example:"26.1"`
	Limit int `query:"limit" default:"100" validate:"min=1,max=1000"`
	Token string `header:"Authorization" validate:"required"`
	Trace string `header:"X-Trace-ID"`
	Session string `cookie:"session"`
	Name string `json:"name" validate:"required"`
	Priority int `json:"priority" default:"3" validate:"min=1,max=5"`
}
type docOutput struct {
	httpapi.ApiResult
	Items []*docItem `json:"items"`
}
type docItem struct {
	Name string `json:"name"`
	Children []*docItem `json:"children,omitempty"`
}

func TestOpenAPI(t *testing.T) {
	s := httpapi.NewWithConfig(httpapi.Config{OpenAPI:&httpapi.OpenAPIConfig{Title:"Release API", SwaggerUI:true}})
	httpapi.POST(s, "/releases/:release/tasks", func(in *docInput, out *docOutput) error { return nil },
		httpapi.Summary("Create task"), httpapi.Tags("Tasks"), httpapi.BearerAuth(),
		httpapi.Response(201,"Created"), httpapi.Response(409,"Conflict"))
	httpapi.GET(s,"/health",func(*httpapi.Empty,*docOutput) error{return nil},httpapi.HideFromOpenAPI())
	raw,err:=s.OpenAPIJSON()
	if err!=nil {t.Fatal(err)}
	var doc map[string]any
	if err=json.Unmarshal(raw,&doc);err!=nil{t.Fatal(err)}
	paths:=doc["paths"].(map[string]any)
	if len(paths)!=1 {t.Fatalf("unexpected paths: %v",paths)}
	op:=paths["/releases/{release}/tasks"].(map[string]any)["post"].(map[string]any)
	if op["summary"]!="Create task" {t.Fatal("summary missing")}
	params:=op["parameters"].([]any)
	if len(params)!=4 {t.Fatalf("unexpected parameters: %v",params)}
	for _,p:=range params{
		p:=p.(map[string]any)
		if p["name"]=="limit" {
			schema:=p["schema"].(map[string]any)
			if schema["default"]!=float64(100)||schema["minimum"]!=float64(1)||schema["maximum"]!=float64(1000){t.Fatal(schema)}
		}
		if p["name"]=="Authorization" {t.Fatal("reserved header documented as parameter")}
	}
	body:=op["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	props:=body["properties"].(map[string]any)
	if len(props)!=2 || props["name"]==nil || props["priority"]==nil {t.Fatalf("wrong JSON properties %v",props)}
	responses:=op["responses"].(map[string]any)
	for _, code:=range []string{"201","400","409","413","415","500"} {if responses[code]==nil{t.Fatalf("missing %s",code)}}
	if responses["200"]!=nil {t.Fatal("explicit success response did not replace default")}
	if !strings.Contains(string(raw),`"alerts"`)||!strings.Contains(string(raw),`"bearerAuth"`)||!strings.Contains(string(raw),`"children"`){t.Fatal("missing output/security schema")}

	for _,path:=range []string{"/openapi.json","/openapi.yaml","/swagger/","/swagger/init.js","/swagger/swagger-ui-bundle.js"}{
		w:=httptest.NewRecorder()
		s.ServeHTTP(w,httptest.NewRequest("GET",path,nil))
		if w.Code!=200{t.Fatalf("%s returned %d",path,w.Code)}
		if path=="/openapi.yaml"{
			var y map[string]any
			if err:=yaml.Unmarshal(w.Body.Bytes(),&y);err!=nil||y["openapi"]!="3.1.0"{t.Fatalf("invalid yaml: %v",err)}
		}
	}
}

func TestOpenAPIDisabled(t *testing.T){
	s:=httpapi.New()
	if _,err:=s.OpenAPIJSON();err==nil{t.Fatal("expected disabled error")}
	w:=httptest.NewRecorder()
	s.ServeHTTP(w,httptest.NewRequest("GET","/openapi.json",nil))
	if w.Code!=404{t.Fatalf("docs enabled by default: %d",w.Code)}
}
