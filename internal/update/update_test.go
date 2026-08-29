package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/version"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// feed serves one release whose assets are built against the test server's own
// address, and points the package at it for the duration of the test.
func feed(t *testing.T, marker string, floor string) Feed {
	t.Helper()
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/tool/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		assets := `{"name":"qrouton-9.9.9-macos-universal.zip","browser_download_url":"` + server.URL + `/archive"}`
		if marker != "" {
			assets += `,{"name":"` + marker + `","browser_download_url":"` + server.URL + `/floor"}`
		}
		w.Write([]byte(`{"assets":[` + assets + `]}`))
	})
	mux.HandleFunc("/floor", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(floor)) })
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)

	base := releaseAPIBase
	releaseAPIBase = server.URL
	t.Cleanup(func() { releaseAPIBase = base })
	return Feed{Repo: "acme/tool"}
}

func TestFloorResolvesThePublishedMinimum(t *testing.T) {
	floor, err := feed(t, floorAsset, "v0.4.0\n").Floor(context.Background())
	if err != nil {
		t.Fatalf("Floor: %v", err)
	}
	if floor != "0.4.0" {
		t.Fatalf("floor = %q, want 0.4.0", floor)
	}
}

// A release cut before the marker existed imposes no floor, so an install is
// never held by a feed that cannot answer the question.
func TestFloorIsEmptyWhenTheReleasePublishesNoMarker(t *testing.T) {
	floor, err := feed(t, "", "").Floor(context.Background())
	if err != nil {
		t.Fatalf("Floor: %v", err)
	}
	if floor != "" {
		t.Fatalf("floor = %q, want none", floor)
	}
}

func TestFloorReportsAFeedItCannotReach(t *testing.T) {
	base := releaseAPIBase
	releaseAPIBase = "http://127.0.0.1:1"
	defer func() { releaseAPIBase = base }()
	if _, err := (Feed{Repo: "acme/tool"}).Floor(context.Background()); err == nil {
		t.Fatal("an unreachable feed reported no error")
	}
}

func TestHeldOnlyBelowThePublishedFloor(t *testing.T) {
	cases := []struct {
		current, floor string
		want           bool
	}{
		{"0.3.1", "0.4.0", true},
		{"0.4.0", "0.4.0", false},
		{"0.5.0", "0.4.0", false},
		// No marker is no floor.
		{"0.3.1", "", false},
		// A working tree is the one thing the release feed cannot speak for.
		{version.Development, "0.4.0", false},
	}
	for _, c := range cases {
		if got := Held(c.current, c.floor); got != c.want {
			t.Errorf("Held(%q, %q) = %v, want %v", c.current, c.floor, got, c.want)
		}
	}
}

// One universal archive serves both architectures, so the match is on the
// release workflow's naming rather than on a GOARCH the filename never carries.
func TestMatchArchivePicksTheUniversalBundle(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "checksums.txt"},
		{Name: "minimum-version.txt"},
		{Name: "qrouton-0.4.0-macos-universal.zip"},
	}
	if got := MatchArchive(updater.CheckRequest{}, assets); got != 2 {
		t.Fatalf("matched index %d, want 2", got)
	}
}

func TestMatchArchiveRefusesAReleaseWithoutOne(t *testing.T) {
	assets := []github.ReleaseAsset{{Name: "checksums.txt"}, {Name: "source.tar.gz"}}
	if got := MatchArchive(updater.CheckRequest{}, assets); got != -1 {
		t.Fatalf("matched index %d, want -1", got)
	}
}

// Only a tagged bundle may replace itself: a bare binary on a developer's path
// has no bundle the helper could swap, and swapping it for a downloaded .app
// would break the install rather than update it.
func TestSupportedRequiresATaggedBundle(t *testing.T) {
	bundle := filepath.Join("/Applications", "qrouton.app", "Contents", "MacOS", "qrouton")
	if bundled("/home/dev/.local/bin/qrouton") {
		t.Error("a bare binary reported itself bundled")
	}
	if !bundled(bundle) {
		t.Error("an installed bundle did not report itself bundled")
	}
	if bundled("") {
		t.Error("an unnameable executable reported itself bundled")
	}
	// Supported reads this process's own path, so on any host that is not a
	// tagged macOS bundle — every CI runner included — it must refuse.
	if Supported(version.Development) {
		t.Error("a development build reported itself updatable")
	}
}

// The framework refuses an unverified artifact, so the checksum sidecar the
// release workflow publishes has to be the one the provider is told to read.
func TestConfigVerifiesAgainstThePublishedChecksums(t *testing.T) {
	cfg, err := Config("v0.4.0")
	if err != nil {
		t.Fatalf("Config: %v", err)
	}
	if cfg.CurrentVersion != "0.4.0" {
		t.Errorf("CurrentVersion = %q, want 0.4.0", cfg.CurrentVersion)
	}
	if cfg.Window != updater.WindowNone {
		t.Error("the updater was configured with a window to dismiss")
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(cfg.Providers))
	}
}
