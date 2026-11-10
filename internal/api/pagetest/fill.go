// Package pagetest checks that the page JSON a message gives matches what the
// Go type it stands for gave. Both fillers derive each leaf from its JSON key
// path, so the same shape fills to the same JSON.
package pagetest

import (
	"encoding/json"
	"hash/fnv"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const mapKey = "key"

func number(path string) uint64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(path))
	return uint64(h.Sum32()%97) + 1
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

var timeType = reflect.TypeFor[time.Time]()

// FillGo sets every field encoding/json writes, through the pointer v.
func FillGo(v any) {
	fillGo(reflect.ValueOf(v).Elem(), "")
}

func fillGo(v reflect.Value, path string) {
	switch {
	case v.Type() == timeType:
		v.Set(reflect.ValueOf(time.Unix(int64(number(path)), 0).UTC()))
		return
	case v.Kind() == reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillGo(v.Elem(), path)
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(path)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(number(path)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(number(path))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(number(path)))
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			v.SetBytes([]byte(path))
			return
		}
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillGo(v.Index(0), path)
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		elem := reflect.New(v.Type().Elem()).Elem()
		fillGo(elem, join(path, mapKey))
		v.SetMapIndex(reflect.ValueOf(mapKey).Convert(v.Type().Key()), elem)
	case reflect.Struct:
		fillStruct(v, path)
	}
}

func fillStruct(v reflect.Value, path string) {
	for i := range v.NumField() {
		field := v.Type().Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" || (!field.IsExported() && !field.Anonymous) {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if field.Anonymous && name == "" {
			inner := v.Field(i)
			if inner.Kind() == reflect.Pointer {
				inner.Set(reflect.New(inner.Type().Elem()))
				inner = inner.Elem()
			}
			fillStruct(inner, path)
			continue
		}
		if name == "" {
			name = field.Name
		}
		fillGo(v.Field(i), join(path, name))
	}
}

// FillProto sets every field protojson writes, skipping the arms of a oneof.
func FillProto(m proto.Message) {
	fillMessage(m.ProtoReflect(), "")
}

func fillMessage(m protoreflect.Message, path string) {
	if m.Descriptor().FullName() == "google.protobuf.Timestamp" {
		m.Set(m.Descriptor().Fields().ByName("seconds"), protoreflect.ValueOfInt64(int64(number(path))))
		return
	}
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if oneof := fd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
			continue
		}
		key := join(path, fd.JSONName())
		switch {
		case fd.IsList():
			list := m.Mutable(fd).List()
			list.Append(element(list.NewElement(), fd, key))
		case fd.IsMap():
			entries := m.Mutable(fd).Map()
			entries.Set(protoreflect.ValueOfString(mapKey).MapKey(), element(entries.NewValue(), fd.MapValue(), join(key, mapKey)))
		case fd.Kind() == protoreflect.MessageKind:
			fillMessage(m.Mutable(fd).Message(), key)
		default:
			m.Set(fd, scalar(fd, key))
		}
	}
}

func element(blank protoreflect.Value, fd protoreflect.FieldDescriptor, path string) protoreflect.Value {
	if fd.Kind() == protoreflect.MessageKind {
		fillMessage(blank.Message(), path)
		return blank
	}
	return scalar(fd, path)
}

func scalar(fd protoreflect.FieldDescriptor, path string) protoreflect.Value {
	n := number(path)
	switch fd.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(path)
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte(path))
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(true)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(n))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(int64(n))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(n))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(n)
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(n))
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(float64(n))
	case protoreflect.EnumKind:
		return protoreflect.ValueOfEnum(fd.Enum().Values().Get(fd.Enum().Values().Len() - 1).Number())
	}
	panic("pagetest: no fill for " + fd.Kind().String())
}

// Same fails t unless got and want parse to the same JSON value.
func Same(t testing.TB, got, want []byte) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want is not JSON: %v\n%s", err, want)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("page JSON differs\n got: %s\nwant: %s", indent(g), indent(w))
	}
}

func indent(v any) string {
	out, _ := json.MarshalIndent(v, "      ", "  ")
	return string(out)
}
