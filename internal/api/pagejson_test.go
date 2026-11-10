package api

import (
	"encoding/json"
	"path/filepath"
	"testing"

	_ "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/api/pagejson"
	"github.com/kieranajp/qrouton/internal/api/pagetest"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Catches a schema edit that changes what the page sees for an empty value,
// such as a nested message that now arrives as null.
func TestEveryMessageEmptyPageJSONMatchesItsGolden(t *testing.T) {
	empty := map[string]json.RawMessage{}
	protoregistry.GlobalFiles.RangeFilesByPackage("qrouton.v1", func(file protoreflect.FileDescriptor) bool {
		messages := file.Messages()
		for i := range messages.Len() {
			desc := messages.Get(i)
			empty[string(desc.Name())] = pagejson.Marshal(dynamicpb.NewMessage(desc))
		}
		return true
	})
	got, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	pagetest.Golden(t, filepath.Join("testdata", "pagejson", "empty.json"), got)
}
