package sessionpaths

import "testing"

func TestSyncCopyMatchesEachSyncToolPattern(t *testing.T) {
	if SyncCopy("R1-x.md") {
		t.Fatal("a real file is not a sync copy")
	}
	for _, name := range []string{
		"R1-x.sync-conflict-20261012-120000-ABCDEFG.md",
		"~syncthing~R1-x.md.tmp",
		"R1-x (conflicted copy 2026-10-12).md",
		"R1-x (Conflict 2026-10-12).md",
	} {
		if !SyncCopy(name) {
			t.Errorf("%q is a sync copy", name)
		}
	}
}
