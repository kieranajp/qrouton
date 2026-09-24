package desktop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UpdateStatus is what the page draws the button from. Command is set on
// darwin, URL everywhere else; Available is false unless a newer release
// was found.
type UpdateStatus struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Command   string `json:"command"`
	URL       string `json:"url"`
}

// Updates is a Wails service reporting whether the running build is outdated.
// An empty version turns it off: watch never requests, and Status stays at
// its zero-update default.
type Updates struct {
	mu       sync.RWMutex
	status   UpdateStatus
	version  string
	emit     emitter
	client   *http.Client
	endpoint string
}

func newUpdates(version string, emit emitter, client *http.Client, endpoint string) *Updates {
	return &Updates{version: version, emit: emit, client: client, endpoint: endpoint, status: UpdateStatus{Current: version}}
}

func (u *Updates) Status() UpdateStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}

// watch checks once, then on the given interval, until ctx is cancelled.
func (u *Updates) watch(ctx context.Context, interval time.Duration) {
	if u.version == "" {
		return
	}
	u.check(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.check(ctx)
		}
	}
}

// check leaves the status untouched on any network error, non-200, or
// unparseable body — a failed check must never look like "no update".
func (u *Updates) check(ctx context.Context) {
	reqCtx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.endpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set(updateAcceptHeader, updateAcceptValue)
	req.Header.Set(updateUserAgentHeader, updateUserAgentPrefix+u.version)
	resp, err := u.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var release struct {
		TagName string `json:"tag_name"`
		URL     string `json:"html_url"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, updateResponseLimit)).Decode(&release) != nil {
		return
	}
	if !newer(release.TagName, u.version) {
		return
	}
	next := UpdateStatus{Available: true, Current: u.version, Latest: strings.TrimPrefix(release.TagName, "v")}
	next.URL = release.URL
	if next.URL == "" {
		next.URL = updateFallbackURL
	}
	if runtime.GOOS == "darwin" {
		next.Command = updateBrewCommand
	}
	u.setStatus(next)
}

func (u *Updates) setStatus(next UpdateStatus) {
	u.mu.Lock()
	if u.status == next {
		u.mu.Unlock()
		return
	}
	u.status = next
	u.mu.Unlock()
	u.emit(updateEvent, next)
}

var versionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`)

// parseVersion accepts what the package scripts accept, after a leading v is
// stripped: one to three dot-separated numeric parts, padded with zeros.
func parseVersion(raw string) ([3]int, bool) {
	trimmed := strings.TrimPrefix(raw, "v")
	if !versionPattern.MatchString(trimmed) {
		return [3]int{}, false
	}
	var out [3]int
	for i, part := range strings.Split(trimmed, ".") {
		out[i], _ = strconv.Atoi(part)
	}
	return out, true
}

// newer reports whether latest outdates current. A latest that fails to
// parse never counts as newer; a current that fails to (an empty dev build
// version never reaches here) counts as older than any latest that does.
func newer(latest, current string) bool {
	latestVersion, ok := parseVersion(latest)
	if !ok {
		return false
	}
	currentVersion, ok := parseVersion(current)
	if !ok {
		return true
	}
	return latestVersion != currentVersion &&
		(latestVersion[0] > currentVersion[0] ||
			latestVersion[0] == currentVersion[0] && latestVersion[1] > currentVersion[1] ||
			latestVersion[0] == currentVersion[0] && latestVersion[1] == currentVersion[1] && latestVersion[2] > currentVersion[2])
}
