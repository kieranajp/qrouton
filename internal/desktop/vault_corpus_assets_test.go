package desktop

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/kieranajp/qrouton/internal/vault"
)

func corpusAssetFixture(t *testing.T, body string) (*Settings, VaultCorpus, string, []byte) {
	t.Helper()
	s, source, _ := importFixture(t)
	root := t.TempDir()
	directory := filepath.Join(root, "example", "shared", "research")
	assets := filepath.Join(root, "example", "assets")
	for _, dir := range []string{directory, assets} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := vault.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc.Body = body
	raw, err = vault.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "R2.md"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(assets, "screen (1).png")
	if err = os.WriteFile(asset, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(assets, "unreferenced.png"), encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	s.chooseImportCorpus = func() (string, error) { return root, nil }
	corpus, err := s.SelectVaultImportCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return s, corpus, asset, encoded.Bytes()
}
func TestVaultCorpusAssetsAdmitReferencesAndRevalidate(t *testing.T) {
	s, corpus, asset, data := corpusAssetFixture(t, "# Evidence\n\n[Capture](../../assets/screen%20%281%29.png)\n\n![Capture](<../../assets/screen \\(1\\).png>)\n")
	preview, err := s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"})
	if err != nil || preview.Ready != 1 {
		t.Fatal(preview, err)
	}
	entry, err := s.VaultCorpusEntry(preview.ID, preview.Entries[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Assets) != 2 || entry.Assets[0].Hash != entry.Assets[1].Hash || entry.Assets[0].Key != entry.Assets[1].Key || preview.Entries[0].AssetCount != 2 {
		t.Fatal(entry.Assets, preview)
	}
	if !strings.Contains(entry.Canonical, "assets/"+entry.Assets[0].Hash+".png") {
		t.Fatal(entry.Canonical)
	}
	for _, file := range s.vaults.importSelections.files {
		if strings.HasSuffix(file.path, "unreferenced.png") {
			t.Fatal("unreferenced asset admitted")
		}
	}
	if err = os.WriteFile(asset, append(data, 0), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmVaultImport(preview.ID, []string{entry.Key}); err == nil {
		t.Fatal("changed PNG accepted")
	}
	if err = os.WriteFile(asset, data, 0600); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmVaultImport(preview.ID, []string{entry.Key}); err != nil {
		t.Fatal(err)
	}
}
func TestVaultCorpusAssetsRejectUnsafeFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "invalid", "large"} {
		t.Run(kind, func(t *testing.T) {
			s, corpus, asset, data := corpusAssetFixture(t, "[Capture](../../assets/screen%20%281%29.png)")
			if err := os.Remove(asset); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				target := filepath.Join(t.TempDir(), "target.png")
				if err := os.WriteFile(target, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, asset); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(asset, 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid":
				if err := os.WriteFile(asset, []byte("not a PNG"), 0600); err != nil {
					t.Fatal(err)
				}
			case "large":
				if err := os.WriteFile(asset, make([]byte, (8<<20)+1), 0600); err != nil {
					t.Fatal(err)
				}
			}
			preview, err := s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"})
			if err != nil || preview.Ready != 0 || preview.Entries[0].AssetCount != 0 {
				t.Fatal(preview, err)
			}
		})
	}
}
func TestVaultCorpusAssetPathBoundary(t *testing.T) {
	source := "example/shared/plans/P1.md"
	for _, href := range []string{"/example/assets/a.png", "../../../other/assets/a.png", "../../assets/../../../escape.png", "../../assets/%2e%2e/%2e%2e/other/assets/a.png", "../../assets/a%5cb.png", "file:///example/assets/a.png", "https://example.com/a.png", "../../assets/a.png?x=y"} {
		if value, ok := corpusAssetPath(source, href); ok {
			t.Fatal(href, value)
		}
	}
	if value, ok := corpusAssetPath(source, "../../assets/a%20b.png#detail"); !ok || value != filepath.FromSlash("example/assets/a b.png") {
		t.Fatal(value, ok)
	}
}

func TestVaultCorpusAssetsRejectReplacementBeforeConfirm(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			s, corpus, asset, data := corpusAssetFixture(t, "[Capture](../../assets/screen%20%281%29.png)")
			preview, err := s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"})
			if err != nil || preview.Ready != 1 {
				t.Fatal(preview, err)
			}
			if err = os.Remove(asset); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				target := filepath.Join(t.TempDir(), "same.png")
				if err = os.WriteFile(target, data, 0600); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(target, asset)
			} else {
				err = syscall.Mkfifo(asset, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ConfirmVaultImport(preview.ID, []string{preview.Entries[0].Key}); err == nil {
				t.Fatal("replacement admitted")
			}
		})
	}
}
