package pagetest

import (
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
)

type inner struct {
	Label string `json:"label"`
}

type embedded struct {
	Flat string `json:"flat"`
}

type shape struct {
	embedded
	Name    string            `json:"name"`
	Count   int               `json:"count,omitempty"`
	Rev     uint64            `json:"rev"`
	On      bool              `json:"on"`
	Tags    []string          `json:"tags"`
	Inner   inner             `json:"inner"`
	Items   []inner           `json:"items"`
	Kind    string            `json:"kind,omitempty"`
	Data    []byte            `json:"encoded"`
	At      time.Time         `json:"at"`
	Ratio   float64           `json:"ratio"`
	Labels  map[string]string `json:"labels"`
	Ignored string            `json:"-"`
	Spelled string            `json:"bugReportID"`
}

// shapeMessage is shape written as a message, built at run time so the test
// carries no .proto of its own.
func shapeMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
	optional := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	repeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	field := func(name string, number int32, kind *descriptorpb.FieldDescriptorProto_Type, label *descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
		return &descriptorpb.FieldDescriptorProto{Name: &name, Number: &number, Type: kind, Label: label}
	}
	ofType := func(f *descriptorpb.FieldDescriptorProto, typeName string) *descriptorpb.FieldDescriptorProto {
		f.TypeName = &typeName
		return f
	}
	kind := field("kind", 9, str, optional)
	kind.Proto3Optional = new(true)
	kind.OneofIndex = new(int32(0))
	data := field("data", 10, descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(), optional)
	data.JsonName = new("encoded")
	spelled := field("bug_report_id", 14, str, optional)
	spelled.JsonName = new("bugReportID")
	file := &descriptorpb.FileDescriptorProto{
		Name:       new("shape.proto"),
		Package:    new("pagetest"),
		Syntax:     new("proto3"),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: new("Inner"), Field: []*descriptorpb.FieldDescriptorProto{field("label", 1, str, optional)}},
			{
				Name: new("Shape"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("flat", 15, str, optional),
					field("name", 1, str, optional),
					field("count", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(), optional),
					field("rev", 3, descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum(), optional),
					field("on", 4, descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(), optional),
					field("tags", 5, str, repeated),
					ofType(field("inner", 6, msg, optional), ".pagetest.Inner"),
					ofType(field("items", 7, msg, repeated), ".pagetest.Inner"),
					kind,
					data,
					ofType(field("at", 11, msg, optional), ".google.protobuf.Timestamp"),
					field("ratio", 12, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(), optional),
					ofType(field("labels", 13, msg, repeated), ".pagetest.Shape.LabelsEntry"),
					spelled,
				},
				NestedType: []*descriptorpb.DescriptorProto{{
					Name:    new("LabelsEntry"),
					Field:   []*descriptorpb.FieldDescriptorProto{field("key", 1, str, optional), field("value", 2, str, optional)},
					Options: &descriptorpb.MessageOptions{MapEntry: new(true)},
				}},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: new("_kind")}},
			},
		},
	}
	fd, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}
	return dynamicpb.NewMessage(fd.Messages().ByName("Shape"))
}

// Catches a filler that drifts from the other, which would leave every page
// JSON comparison passing or failing on the filler rather than the schema.
func TestTheFillersWriteTheSameJSONForTheSameShape(t *testing.T) {
	var value shape
	FillGo(&value)
	want, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	message := shapeMessage(t)
	FillProto(message)
	got, err := protojson.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	Same(t, got, want)
}
