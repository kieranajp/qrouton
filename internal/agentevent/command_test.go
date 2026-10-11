package agentevent

import "testing"

func TestTrimCommand(t *testing.T) {
	const cwd, root = "/s/x/src/qrouton", "/s/x"
	for _, tc := range []struct{ name, in, want string }{
		{"cd with &&", "cd /s/x/src/qrouton && go test ./...", "go test ./..."},
		{"cd with ;", "cd /s/x/src/qrouton; make check", "make check"},
		{"cd into the session", "cd /s/x/src/web && npm test", "npm test"},
		{"cd elsewhere", "cd /tmp/other && ls", "cd /tmp/other && ls"},
		{"quoted cd", `cd "/s/x/src/qrouton" && ls`, "ls"},
		{"paths under cwd", "go test /s/x/src/qrouton/internal/x", "go test internal/x"},
		{"paths under the session", "cat /s/x/thoughts/shared/R1.md /s/x/src/web/a.ts", "cat thoughts/shared/R1.md web/a.ts"},
		{"path outside both", "cat /etc/hosts", "cat /etc/hosts"},
		{"similar prefix untouched", "ls /s/xy/a", "ls /s/xy/a"},
		{"no paths", "git status", "git status"},
		{"bare cd", "cd /s/x/src/qrouton &&", "cd qrouton &&"},
	} {
		if got := TrimCommand(tc.in, cwd, root); got != tc.want {
			t.Errorf("%s: TrimCommand(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}
