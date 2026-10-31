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

func TestArtifactKindsHaveDedicatedColours(t *testing.T) {
	want := map[string]string{
		RoleArtifactPlan: Lavender, RoleArtifactSpec: Pink,
		RoleArtifactResearch: Sky, RoleArtifactNote: Flamingo,
		RoleArtifactExplainer: Maroon, RoleArtifactReview: Mauve,
		RoleArtifactAudit: Rosewater,
	}
	seen := map[string]string{}
	for role, colour := range want {
		if Roles[role] != colour {
			t.Errorf("%s = %q, want %q", role, Roles[role], colour)
		}
		if other := seen[colour]; other != "" {
			t.Errorf("%s and %s share %s", role, other, colour)
		}
		seen[colour] = role
	}
	for role, colour := range Roles {
		if colour == Rosewater && role != RoleArtifactAudit {
			t.Errorf("%s borrows the audit's %s", role, colour)
		}
	}
}

func TestStickersHaveFixedDedicatedColours(t *testing.T) {
	want := map[string]string{
		"sticker-blue": "ctp-blue", "sticker-sapphire": "ctp-sapphire", "sticker-teal": "ctp-teal",
		"sticker-green": "ctp-green", "sticker-yellow": "ctp-yellow", "sticker-orange": "ctp-peach",
		"sticker-red": "ctp-red", "sticker-maroon": "ctp-maroon", "sticker-pink": "ctp-pink",
		"sticker-mauve": "ctp-mauve", "sticker-lavender": "ctp-lavender", "sticker-grey": "ctp-subtext-0",
	}
	css := CSS()
	for alias, palette := range want {
		declaration := "  --" + alias + ": var(--" + palette + ");"
		if !strings.Contains(css, declaration) {
			t.Errorf("stylesheet is missing %q", declaration)
		}
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
