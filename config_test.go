package main

import (
	"reflect"
	"testing"
)

func TestSplitOrgsTrimsDeduplicatesAndDropsEmptyValues(t *testing.T) {
	got := splitOrgs(" lifesum, second-org, lifesum, ,third ")
	want := []string{"lifesum", "second-org", "third"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitOrgs() = %#v, want %#v", got, want)
	}
}

func TestRepoIDIncludesOrganization(t *testing.T) {
	if got := repoID(Repo{Org: "acme", Name: "api"}); got != "acme/api" {
		t.Fatalf("repoID() = %q", got)
	}
}
