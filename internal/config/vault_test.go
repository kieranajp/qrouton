package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestVaultInstallationIdentityIsStableAcrossConcurrentInitialization(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	results := make(chan VaultStateInfo, 12)
	failures := make(chan error, 12)
	var group sync.WaitGroup
	for range 12 {
		group.Go(func() { state, err := VaultState(); results <- state; failures <- err })
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first VaultStateInfo
	for state := range results {
		if first.InstallationID == "" {
			first = state
		}
		if state != first {
			t.Fatalf("unstable identity: %+v / %+v", first, state)
		}
	}
	if len(first.InstallationID) != 32 {
		t.Fatal(first)
	}
	info, err := os.Stat(filepath.Join(first.Directory, vaultInstallationFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	if err := os.WriteFile(filepath.Join(first.Directory, vaultInstallationFile), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := VaultState(); err == nil {
		t.Fatal("corrupt identity was replaced")
	}
}
