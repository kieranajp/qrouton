package version

import "testing"

func TestBeforeOrdersReleases(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.3.1", "0.4.0", true},
		{"0.4.0", "0.3.1", false},
		{"0.4.0", "0.4.0", false},
		{"0.9.0", "0.10.0", true},
		{"1.0.0", "0.99.9", false},
		// A tag and a bundle version name the same release.
		{"v0.3.1", "0.4.0", true},
		{"0.3.1", "v0.3.1", false},
		// A missing segment reads as zero rather than as an ordering of its own.
		{"0.4", "0.4.0", false},
		{"0.4.0", "0.4", false},
	}
	for _, c := range cases {
		if got := Before(c.a, c.b); got != c.want {
			t.Errorf("Before(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// A working tree is below every release, so a developer is never told they are
// ahead of the latest tag — and above every floor, so the gate never holds one.
func TestDevelopmentOrdersBelowEveryRelease(t *testing.T) {
	if !Before(Development, "0.0.1") {
		t.Error("a development build did not order below a release")
	}
	if Released(Development) {
		t.Error("a development build reported itself released")
	}
	if !Released("0.3.1") {
		t.Error("a stamped version did not report itself released")
	}
}

// An unparseable version orders first, so a garbled release feed cannot present
// itself as newer than the running build.
func TestUnparseableVersionsOrderFirst(t *testing.T) {
	if Before("0.3.1", "not-a-version") {
		t.Error("a garbled version ordered ahead of a release")
	}
	if !Before("", "0.0.1") {
		t.Error("an empty version did not order below a release")
	}
}
