package api

import (
	"encoding/json"
	"path/filepath"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/api/pagejson"
	"github.com/kieranajp/qrouton/internal/api/pagetest"
	"github.com/kieranajp/qrouton/internal/core"
	"google.golang.org/protobuf/proto"
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

// Catches an edit that drops optional from Document.kind or
// TerminalOutput.exit_code: a zero there reads as a note kind, or as a live
// terminal that has exited. protojson leaves an unset optional out, as
// omitempty did.
func TestPageJSONLeavesAnUnsetKindOrExitCodeOut(t *testing.T) {
	for _, tc := range []struct {
		message proto.Message
		key     string
	}{
		{&qroutonv1.Document{}, "kind"},
		{&qroutonv1.TerminalOutput{}, "exit_code"},
	} {
		var page map[string]any
		if err := json.Unmarshal(pagejson.Marshal(tc.message), &page); err != nil {
			t.Fatal(err)
		}
		if value, ok := page[tc.key]; ok && value != nil {
			t.Fatalf("%T reached the page with %q = %v, want it unset", tc.message, tc.key, value)
		}
	}
}

// Catches a document whose invalid UTF-8 fails the wire, or reaches the page
// unlike encoding/json, which replaces each bad byte.
func TestPageJSONOfADocumentWithInvalidUTF8MatchesBefore(t *testing.T) {
	doc := core.Document{Text: "ok \xff\xfe then \xe2\x82 end", Kind: "note"}
	want, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	mapped := document(doc)
	if _, err := proto.Marshal(mapped); err != nil {
		t.Fatalf("the mapped document cannot cross the wire: %v", err)
	}
	var got, before map[string]any
	if err := json.Unmarshal(pagejson.Marshal(mapped), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &before); err != nil {
		t.Fatal(err)
	}
	if got["text"] != before["text"] {
		t.Fatalf("text reached the page as %q, want %q", got["text"], before["text"])
	}
}
