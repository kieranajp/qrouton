package mcpserver

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
)

func TestSetDigestWritesOnlyTheDigestAndKeepsItsLines(t *testing.T) {
	dir := summarySession(t)
	before := rawManifest(t, dir)
	start := time.Now().UTC()

	text := "Research answered the PR questions.\nPlan is written.\nWaiting on the user to approve it."
	if _, err := setDigest(dir, setDigestInput{Digest: "\n  " + text + "  \n"}); err != nil {
		t.Fatalf("setDigest: %v", err)
	}

	after := rawManifest(t, dir)
	if _, ok := before["digest"]; ok {
		t.Fatal("fixture already carries a digest")
	}
	delete(after, "digest")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("fields other than digest changed:\nbefore %s\nafter  %s", before, after)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Digest.Text != text {
		t.Errorf("text = %q, want %q", m.Digest.Text, text)
	}
	if m.Digest.At.Before(start.Truncate(time.Second)) || m.Digest.At.Location() != time.UTC {
		t.Errorf("at = %v, want UTC at or after %v", m.Digest.At, start)
	}
}

func TestSetDigestReplacesTheWholeDigest(t *testing.T) {
	dir := summarySession(t)
	for _, text := range []string{"First.", "Second."} {
		if _, err := setDigest(dir, setDigestInput{Digest: text}); err != nil {
			t.Fatal(err)
		}
	}
	if m, err := session.Load(dir); err != nil || m.Digest.Text != "Second." {
		t.Fatalf("digest = %q, %v", m.Digest.Text, err)
	}
}

func TestSetDigestHoldsTheLimitInRunes(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		ok   bool
	}{
		{"exactly the limit", strings.Repeat("a", digestLimit), true},
		{"one over", strings.Repeat("a", digestLimit+1), false},
		{"multibyte at the limit", strings.Repeat("é", digestLimit), true},
		{"multibyte one over", strings.Repeat("é", digestLimit+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := summarySession(t)
			_, err := setDigest(dir, setDigestInput{Digest: tc.text})
			if tc.ok {
				if err != nil {
					t.Fatalf("setDigest: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted text over the limit")
			}
			for _, want := range []string{strconv.Itoa(digestLimit + 1), strconv.Itoa(digestLimit)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %s", err, want)
				}
			}
			if _, ok := rawManifest(t, dir)["digest"]; ok {
				t.Error("a rejected digest was written")
			}
		})
	}
}

func TestSetDigestRejectsBlankText(t *testing.T) {
	dir := summarySession(t)
	if _, err := setDigest(dir, setDigestInput{Digest: " \n\t "}); !errors.Is(err, ErrDigestRequired) {
		t.Fatalf("err = %v, want %v", err, ErrDigestRequired)
	}
	if _, ok := rawManifest(t, dir)["digest"]; ok {
		t.Error("a blank digest was written")
	}
}

func TestDigestSchemaAndDescriptionNameTheLimit(t *testing.T) {
	field, _ := reflect.TypeOf(setDigestInput{}).FieldByName("Digest")
	for _, text := range []string{field.Tag.Get("jsonschema"), descSetDigest} {
		if !strings.Contains(text, strconv.Itoa(digestLimit)) {
			t.Errorf("%q does not name %d", text, digestLimit)
		}
	}
}
