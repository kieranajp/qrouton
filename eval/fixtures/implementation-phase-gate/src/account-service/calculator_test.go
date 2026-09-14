package account

import "testing"

func TestCalculatorAdd(t *testing.T) {
	if got := (Calculator{}).Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
