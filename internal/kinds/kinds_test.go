package kinds

import (
	"slices"
	"testing"
)

func TestTemplatedLeavesOutExplainer(t *testing.T) {
	got := Templated()
	if !slices.Equal(got, []Kind{Research, Spec, Plan}) {
		t.Fatalf("Templated() = %v", got)
	}
}

func TestRowsAreDistinct(t *testing.T) {
	dirs, names, letters := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, k := range All() {
		if dirs[k.Dir] || names[k.Name] || letters[k.Letter] {
			t.Fatalf("%+v repeats a folder, name or letter", k)
		}
		dirs[k.Dir], names[k.Name], letters[k.Letter] = true, true, true
	}
}
