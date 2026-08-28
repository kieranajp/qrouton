package diagram

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// An unset field leaves d2's light default showing through in one place, which
// reads as a rendering bug rather than a missing override.
func TestEveryOverrideIsSetToHex(t *testing.T) {
	value := reflect.ValueOf(*overrides())
	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		field := value.Field(i)
		if field.IsNil() {
			t.Errorf("%s is unset, so d2's own colour survives", name)
			continue
		}
		if _, err := brightness(field.Elem().String()); err != nil {
			t.Errorf("%s = %q, which is not a hex colour", name, field.Elem().String())
		}
	}
	if value.NumField() != 18 {
		t.Errorf("ThemeOverrides has %d fields; the mapping covers 18", value.NumField())
	}
}

// d2 numbers its neutrals ink first, so on a dark palette N1 must be the
// lightest and N7 the darkest. Reversed, the diagram looks fine at a glance and
// has its labels and its background swapped.
func TestNeutralRampRunsLightToDark(t *testing.T) {
	set := overrides()
	ramp := []*string{set.N1, set.N2, set.N3, set.N4, set.N5, set.N6, set.N7}
	previous := 256.0
	for at, colour := range ramp {
		level, err := brightness(*colour)
		if err != nil {
			t.Fatal(err)
		}
		if level >= previous {
			t.Errorf("N%d (%s) is not darker than N%d", at+1, *colour, at)
		}
		previous = level
	}
}

func brightness(colour string) (float64, error) {
	digits, ok := strings.CutPrefix(colour, "#")
	if !ok || len(digits) != 6 {
		return 0, strconv.ErrSyntax
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return 0, err
	}
	r, g, b := float64(value>>16&0xff), float64(value>>8&0xff), float64(value&0xff)
	return 0.2126*r + 0.7152*g + 0.0722*b, nil
}
