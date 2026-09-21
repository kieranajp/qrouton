package mcpserver

import (
	"context"
	"errors"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type vaultHost struct {
	fakeHost
	status      vault.Status
	ref         vault.Reference
	unavailable bool
}

func (h *vaultHost) VaultStatus(context.Context) (vault.Status, error) { return h.status, nil }
func (h *vaultHost) ReadVault(_ context.Context, ref vault.Reference) (vault.ReadResult, error) {
	h.ref = ref
	if h.unavailable {
		return vault.ReadResult{}, vault.ErrUnavailable
	}
	return vault.ReadResult{Reference: ref, Content: "canonical"}, nil
}
func (h *vaultHost) SearchVault(context.Context, workbench.VaultSearchRequest) (workbench.VaultSearchResult, error) {
	return workbench.VaultSearchResult{Status: h.status, Outcome: vault.StateUnavailable, Results: []vault.Reference{}}, nil
}
func TestVaultToolsFollowSessionAccessAndSurviveUnavailableRoot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status vault.Status
		want   bool
	}{
		{name: "unconfigured"},
		{name: "unselected", status: vault.Status{Enabled: true, Scope: vault.ReadScope{SelectionRequired: true}}},
		{name: "missing root", status: vault.Status{Enabled: true, State: vault.StateUnavailable, Scope: vault.ReadScope{Profiles: []string{"shared"}}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &vaultHost{status: tc.status}
			server := newMCPServer(t.TempDir(), testEditor, host, session.ModeRPI)
			names := advertisedTools(t, server)
			if names[toolReadVault] != tc.want || names[toolSearchVault] != tc.want {
				t.Fatalf("%+v", names)
			}
			host.unavailable = true
			after := advertisedTools(t, server)
			if after[toolReadVault] != tc.want || after[toolSearchVault] != tc.want {
				t.Fatal("tools changed after dependency failure")
			}
		})
	}
}
func TestVaultOpenForwardsOnlyTypedReference(t *testing.T) {
	host := &vaultHost{}
	manager := newWindowManager(t.TempDir(), testEditor, host)
	ref := vault.Reference{Profile: "shared", ID: "example/R1"}
	_, _, err := manager.openFile(context.Background(), openFileInput{Vault: &ref})
	if err != nil {
		t.Fatal(err)
	}
	if len(host.opens) != 1 || host.opens[0].Vault == nil || *host.opens[0].Vault != ref || len(host.opens[0].Command) != 0 {
		t.Fatalf("%+v", host.opens)
	}
	if _, _, err := manager.openFile(context.Background(), openFileInput{Path: "file.md", Vault: &ref}); !errors.Is(err, ErrVaultFileChoice) {
		t.Fatal(err)
	}
}
