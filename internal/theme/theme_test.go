package theme

import (
	"strings"
	"testing"
)

// A role missing here is a component drawing with an unset custom property,
// which the browser renders as nothing rather than as an error.
func TestCSSDeclaresEveryRole(t *testing.T) {
	css := CSS()
	for name, value := range Roles {
		want := "  --" + name + ": " + value + ";"
		if !strings.Contains(css, want) {
			t.Errorf("stylesheet is missing %q", want)
		}
	}
	if len(roleOrder) != len(Roles) {
		t.Fatalf("roleOrder has %d names, Roles has %d", len(roleOrder), len(Roles))
	}
}

func TestWaitingIsPeachAndLabelsAreButter(t *testing.T) {
	if Roles[RoleStateWaiting] != Peach {
		t.Errorf("state-waiting = %q, want peach %q", Roles[RoleStateWaiting], Peach)
	}
	if Roles[RoleAccentLabel] != Yellow {
		t.Errorf("accent-label = %q, want butter %q", Roles[RoleAccentLabel], Yellow)
	}
	if Roles[RoleAccentAction] == Roles[RoleAccentLabel] {
		t.Error("acting and naming share a colour, so neither can be trusted")
	}
}

// An unresolved var() reports nothing anywhere.
func TestEveryReferenceResolves(t *testing.T) {
	css := CSS()
	for _, alias := range aliases {
		name, ok := strings.CutPrefix(alias.value, cssRefOpen)
		if !ok {
			continue
		}
		name = strings.TrimSuffix(name, cssRefClose)
		if !strings.Contains(css, "  --"+name+": ") {
			t.Errorf("--%s refers to --%s, which nothing declares", alias.name, name)
		}
	}
}
