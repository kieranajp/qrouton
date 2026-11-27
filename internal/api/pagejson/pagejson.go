// Package pagejson is the JSON the page reads and sends: the proto schema's,
// through protojson.
package pagejson

import (
	"bytes"
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var marshal = protojson.MarshalOptions{EmitUnpopulated: true}

// Marshal writes every field, so the page reads a zero value rather than a
// missing key. A message off the wire holds valid UTF-8, so it cannot fail.
func Marshal(m proto.Message) json.RawMessage {
	out, err := marshal.Marshal(m)
	if err != nil {
		panic(err)
	}
	return out
}

// List writes a repeated field as a bare array, as a method returning a slice did.
func List[M proto.Message](items []M) json.RawMessage {
	var out bytes.Buffer
	out.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			out.WriteByte(',')
		}
		out.Write(Marshal(item))
	}
	out.WriteByte(']')
	return out.Bytes()
}

// Unmarshal refuses a key the schema does not know, which from the page is a bug.
func Unmarshal(data json.RawMessage, m proto.Message) error {
	return protojson.Unmarshal(data, m)
}
