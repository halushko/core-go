package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})
var rawMessageType = reflect.TypeOf(json.RawMessage{})
var jsonMarshaler = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
var jsonUnmarshaler = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()

func nullableSchema(schema object) object {
	// Keep annotations on the outer schema, including the typed default.
	if _, ok := schema["anyOf"]; ok { return schema }
	if t, ok := schema["type"].(string); ok {
		schema["type"] = []string{t, "null"}
		if values, ok := schema["enum"].([]any); ok { schema["enum"] = append(values, nil) }
		return schema
	}
	if _, ok := schema["$ref"]; ok {
		result := object{"anyOf":[]object{schema, {"type":"null"}}}
		for _, key := range []string{"default", "description", "example"} {
			if value, ok := schema[key]; ok { result[key] = value; delete(schema,key) }
		}
		return result
	}
	return schema
}

func (d *openAPIDocument) typeSchema(t reflect.Type, input, text bool) object {
	if t.Kind() == reflect.Pointer {
		s := d.typeSchema(t.Elem(), input, text)
		if !text { return nullableSchema(s) }
		return s
	}
	if t == timeType { return object{"type":"string", "format":"date-time"} }
	if text && reflect.PointerTo(t).Implements(textUnmarshaler) { return object{"type":"string"} }
	if t == rawMessageType { return object{} }
	// Custom JSON encoders/decoders may use an arbitrary wire format.
	if !text && ((!input && (t.Implements(jsonMarshaler) || reflect.PointerTo(t).Implements(jsonMarshaler))) ||
		(input && reflect.PointerTo(t).Implements(jsonUnmarshaler))) {
		return object{"x-go-type":t.String()}
	}
	switch t.Kind() {
	case reflect.String: return object{"type":"string"}
	case reflect.Bool: return object{"type":"boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s := object{"type":"integer"}
		if t.Bits() == 32 { s["format"] = "int32" }; if t.Bits() == 64 { s["format"] = "int64" }
		return s
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return object{"type":"integer", "minimum":0}
	case reflect.Float32: return object{"type":"number", "format":"float"}
	case reflect.Float64: return object{"type":"number", "format":"double"}
	case reflect.Slice:
		if !text && t.Elem().Kind() == reflect.Uint8 { return nullableSchema(object{"type":"string", "contentEncoding":"base64"}) }
		s := object{"type":"array", "items":d.typeSchema(t.Elem(), input, text)}
		if !text { return nullableSchema(s) }; return s
	case reflect.Array:
		return object{"type":"array", "items":d.typeSchema(t.Elem(), input, text), "minItems":t.Len(), "maxItems":t.Len()}
	case reflect.Map:
		return nullableSchema(object{"type":"object", "additionalProperties":d.typeSchema(t.Elem(), input, text)})
	case reflect.Struct:
		// Separate request/response schemas because their required and
		// additionalProperties rules differ. Register placeholder first for recursion.
		mode := "Output"
		if input { mode = "Input" }
		hash := sha256.Sum256([]byte(t.PkgPath() + ":" + t.String()))
		name := t.Name()
		if name == "" { name = "Object" }
		name = fmt.Sprintf("%s_%s_%x", name, mode, hash[:6])
		if _, exists := d.schemas[name]; !exists {
			d.schemas[name] = object{}
			d.schemas[name] = d.structSchema(t, input, map[reflect.Type]bool{})
		}
		return object{"$ref":"#/components/schemas/"+name}
	default: return object{"x-go-type":t.String()}
	}
}

func (d *openAPIDocument) structSchema(t reflect.Type, input bool, embedding map[reflect.Type]bool) object {
	props := make(object)
	required := []string{}
	embedding[t] = true
	defer delete(embedding,t)
	for i:=0;i<t.NumField();i++ {
		f := t.Field(i)
		if f.PkgPath != "" { continue }
		tag := strings.Split(f.Tag.Get("json"), ",")
		if tag[0] == "-" { continue }
		ft := f.Type
		if ft.Kind() == reflect.Pointer { ft = ft.Elem() }
		if f.Anonymous && tag[0] == "" && ft.Kind() == reflect.Struct && !embedding[ft] {
			nested := d.structSchema(ft,input,embedding)
			for key, value := range nested["properties"].(object) { if _, exists := props[key]; !exists { props[key] = value } }
			if ft == f.Type { if r, ok := nested["required"].([]string); ok { required = append(required,r...) } }
			continue
		}
		name := tag[0]
		if name == "" { name = f.Name }
		s := d.fieldSchema(f,input,false)
		optional := false
		for _, flag := range tag[1:] {
			if flag == "omitempty" || flag == "omitzero" { optional = true }
			if flag == "string" {
				switch ft.Kind() {
				case reflect.String,reflect.Bool,reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64,reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64,reflect.Float32,reflect.Float64:
					s = object{"type":"string"}
					if f.Type.Kind()==reflect.Pointer { s = nullableSchema(s) }
				}
			}
		}
		props[name] = s
		if (!input && !optional) || (input && hasRequired(f.Tag.Get("validate"))) { required = append(required,name) }
	}
	s := object{"type":"object","properties":props}
	if input { s["additionalProperties"] = false }
	if len(required) != 0 {
		seen := make(map[string]bool)
		unique := []string{}
		for _, key := range required { if !seen[key] { unique=append(unique,key); seen[key]=true } }
		s["required"] = unique
	}
	return s
}

func hasRequired(tag string) bool {
	for _, rule := range strings.Split(tag,",") { if rule == "required" { return true } }
	return false
}

func (d *openAPIDocument) fieldSchema(f reflect.StructField, input, text bool) object {
	s := d.typeSchema(f.Type,input,text)
	t := f.Type
	for t.Kind()==reflect.Pointer { t=t.Elem() }
	if description := f.Tag.Get("description"); description != "" { s["description"] = description }
	if example, exists := f.Tag.Lookup("example"); exists {
		if v, ok := typedLiteral(t, example); ok { s["example"] = v }
	}
	if !input && !text { return s }
	tag := f.Tag.Get("validate")
	if tag != "" { s["x-go-validate"] = tag }
	// Do not translate complex validator expressions into a stronger contract
	// than the actual validator. Preserve the complete expression as an extension.
	for _, rule := range strings.Split(tag,",") {
		if strings.Contains(rule,"|") || rule == "dive" || rule == "keys" { break }
		name, raw, _ := strings.Cut(rule,"=")
		if name == "oneof" && !strings.Contains(raw,"'") {
			var values []any
			for _, part := range strings.Fields(raw) {
				v, ok := typedLiteral(t,part); if !ok { values=nil; break }; values=append(values,v)
			}
			if len(values)!=0 { s["enum"] = values }
		}
		if name != "min" && name != "max" && name != "len" && name != "gte" && name != "lte" && name != "gt" && name != "lt" { continue }
		n, err := strconv.ParseFloat(raw,64); if err != nil { continue }
		var min,max string
		switch t.Kind() {
		case reflect.String: min,max="minLength","maxLength"
		case reflect.Slice,reflect.Array: min,max="minItems","maxItems"
		case reflect.Map: min,max="minProperties","maxProperties"
		case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64,reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64,reflect.Float32,reflect.Float64: min,max="minimum","maximum"
		default: continue
		}
		switch name {
		case "min","gte": s[min]=n
		case "max","lte": s[max]=n
		case "len": s[min]=n; s[max]=n
		case "gt": if min=="minimum" { s["exclusiveMinimum"]=n } else { s[min]=n+1 }
		case "lt": if max=="maximum" { s["exclusiveMaximum"]=n } else if n>=1 { s[max]=n-1 }
		}
	}
	if hasRequired(tag) && f.Type.Kind()!=reflect.Pointer {
		switch t.Kind() {
		case reflect.String: if _, ok:=s["minLength"]; !ok { s["minLength"]=1 }
		case reflect.Bool: s["const"]=true
		case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64,reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64,reflect.Float32,reflect.Float64:
			s["not"]=object{"const":0}
		}
	}
	return s
}

func typedLiteral(t reflect.Type, raw string) (any,bool) {
	if t.Kind()==reflect.String || t==timeType { return raw,true }
	if t.Kind()==reflect.Bool { v,err:=strconv.ParseBool(raw); return v,err==nil }
	v := reflect.New(t)
	if err:=json.Unmarshal([]byte(raw),v.Interface()); err != nil { return nil,false }
	b,err:=json.Marshal(v.Elem().Interface()); if err!=nil { return nil,false }
	var value any
	err=json.Unmarshal(b,&value)
	return value,err==nil
}
