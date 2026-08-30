// Package update decides whether an install may keep running and, when a newer
// release exists, describes the one it should move to. It owns the release
// feed's two answers — the archive to take, and the oldest version that
// archive will talk to — and nothing about windows or when to apply them.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kieranajp/qrouton/internal/version"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

var releaseAPIBase = apiBase

// Feed reads the release feed for the one fact the framework's provider does
// not carry: the floor a release advertises.
type Feed struct {
	Client *http.Client
	Repo   string
}

// Floor is the oldest version the latest release will talk to, or "" when the
// release advertises none. A release that predates the marker imposes no
// floor, so an old install is never held by a feed that cannot answer.
func (f Feed) Floor(ctx context.Context) (string, error) {
	repo := f.Repo
	if repo == "" {
		repo = Repository
	}
	client := f.Client
	if client == nil {
		// The Keeper bounds this with a context too; a client timeout is what
		// keeps a direct caller from hanging on a socket that never answers.
		client = &http.Client{Timeout: checkTimeout}
	}

	var release struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	endpoint := releaseAPIBase + fmt.Sprintf(latestPath, repo)
	if err := get(ctx, client, endpoint, &release); err != nil {
		return "", err
	}
	for _, asset := range release.Assets {
		if asset.Name != floorAsset {
			continue
		}
		body, err := fetch(ctx, client, asset.URL, assetMediaType, floorLimit)
		if err != nil {
			return "", err
		}
		return version.Trim(string(body)), nil
	}
	return "", nil
}

// Held reports whether an install running current must update before it may be
// used, given the floor the latest release advertises. A development build is
// never held: a working tree is the one thing the release feed cannot speak for.
func Held(current, floor string) bool {
	if floor == "" || !version.Released(current) {
		return false
	}
	return version.Before(current, floor)
}

// Config is what the workbench hands the framework's updater. Providers is one
// GitHub release feed; the artifact is verified against the checksums the
// release workflow publishes beside it, and no window is ever opened — this
// updater has no opinion the user is asked for. Cadence is left to Keeper, so
// the framework's own timer is not a second owner of when a check happens.
func Config(current string) (updater.Config, error) {
	provider, err := github.New(github.Config{
		Repository:    Repository,
		ChecksumAsset: checksumAsset,
		AssetMatcher:  MatchArchive,
	})
	if err != nil {
		return updater.Config{}, err
	}
	return updater.Config{
		CurrentVersion: version.Trim(current),
		Providers:      []updater.Provider{provider},
		Window:         updater.WindowNone,
	}, nil
}

// MatchArchive picks the universal macOS archive. The framework's own matcher
// wants the running GOARCH in the filename and one universal build carries
// both, so a release's single archive would otherwise never be matched.
func MatchArchive(_ updater.CheckRequest, assets []github.ReleaseAsset) int {
	for i, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasPrefix(name, archivePrefix) && strings.HasSuffix(name, archiveSuffix) {
			return i
		}
	}
	return -1
}

// Supported reports whether this install can replace itself: a macOS bundle,
// cut from a tag. A `go build` on a developer's path is neither, and swapping a
// bare binary for a downloaded .app would break the install rather than update
// it. A path this process cannot even name is refused for the same reason.
func Supported(current string) bool {
	if runtime.GOOS != darwinPlatform || !version.Released(current) {
		return false
	}
	executable, err := os.Executable()
	return err == nil && bundled(executable)
}

// bundled reports whether executable lives inside a .app. The framework's
// helper replaces the bundle it finds around the running binary, so an install
// with no bundle around it has nothing the helper may safely replace.
func bundled(executable string) bool {
	if executable == "" {
		return false
	}
	for dir := filepath.Clean(executable); ; {
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		if strings.HasSuffix(parent, appBundleSuffix) {
			return true
		}
		dir = parent
	}
}

func get(ctx context.Context, client *http.Client, url string, into any) error {
	body, err := fetch(ctx, client, url, apiMediaType, releaseLimit)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}

func fetch(ctx context.Context, client *http.Client, url, accept string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(acceptHeader, accept)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoRelease
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// HandleHelper performs the bundle swap when this process was re-executed as
// the updater's helper, and never returns in that case. Every other process
// returns immediately, so it costs an environment lookup on a normal launch.
//
// It has to run before anything else: the helper is the app's own binary with
// sentinel environment variables set, so left to itself it would parse the
// command line and try to be a workbench instead of replacing one.
func HandleHelper() { updater.HandleHelperMode() }
