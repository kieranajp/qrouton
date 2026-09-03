package desktop

import (
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// identity is what a session is called and where it lives. Onboarding chooses
// both after the workbench has opened, so it is replaced rather than mutated.
type identity struct {
	slug string
	root string
}

type sessionState struct {
	// terminal addresses the conversation PTY. Onboarding execs the supervisor
	// inside that PTY, so a slug-keyed stream would go deaf across the handover.
	terminal string
	agents   *agentActivity
	provider string
	argv     []string
	env      []string
	named    atomic.Pointer[identity]
	// control is the session's own listener, and nil for a session whose control
	// arrives on the process socket instead.
	control io.Closer

	mu      sync.Mutex
	stopped bool
	process *ptyProcess
	shell   string
	// picker is the request waiting for the user to arrive at this session.
	// Agent requests expire with their caller; one opened by the user does not.
	picker *workbench.PickerRequest
	// shells counts the shells the session has had rather than the ones still
	// open: a number freed by a close would name two terminals at once.
	shells int
}

// requestPicker queues a picker on this session. A later request replaces an
// earlier one: both pollers then read the one stanza the confirm writes.
func (s *sessionState) requestPicker(req workbench.PickerRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.picker = &req
}

// pendingPicker is this session's picker request while it is still worth drawing.
func (s *sessionState) pendingPicker() *workbench.PickerRequest {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.picker == nil || (!s.picker.Deadline.IsZero() && !time.Now().Before(s.picker.Deadline)) {
		return nil
	}
	return s.picker
}

// clearPicker is confirm and cancel both: arriving at the session again must not
// redraw a picker that has been answered.
func (s *sessionState) clearPicker() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.picker = nil
}

// slug and root tolerate a nil session, which is the window showing none.
func (s *sessionState) slug() string {
	if s == nil {
		return ""
	}
	return s.named.Load().slug
}

func (s *sessionState) root() string {
	if s == nil {
		return ""
	}
	return s.named.Load().root
}

// start launches the supervisor under a PTY sized to the terminal displaying it.
// The page calls it on load, so a reload must not fork a second agent.
func (s *sessionState) start(emit emitter, exited func(*sessionState, int), cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.process != nil {
		return nil
	}
	process, err := startPTY(s.argv, s.env, s.root(), cols, rows)
	if err != nil {
		return err
	}
	s.process = process
	// Base64 because a raw PTY chunk is not valid UTF-8 at its boundary and
	// JSON marshalling corrupts it.
	go process.pump(
		func(b []byte) {
			s.agents.output()
			emit(ptyDataEvent+s.terminal, base64.StdEncoding.EncodeToString(b))
		},
		func(code int) {
			emit(ptyExitEvent+s.terminal, code)
			if exited != nil {
				exited(s, code)
			}
		},
	)
	return nil
}

func (s *sessionState) write(data []byte) error {
	s.agents.input()
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	if process == nil {
		return ErrTerminalNotStarted
	}
	return process.write(data)
}

func (s *sessionState) resize(cols, rows int) error {
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	if process == nil {
		return nil
	}
	return process.resize(cols, rows)
}

// serve records the listener the session answers on, refusing once it has been
// stopped: a listener installed after teardown is one nobody will ever close.
func (s *sessionState) serve(control io.Closer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	s.control = control
	return true
}

// stop ends the session's process tree and closes the listener it was served on.
// Both are cleared under the lock, so neither can be torn down twice.
func (s *sessionState) stop() {
	s.mu.Lock()
	s.stopped = true
	process, control := s.process, s.control
	s.process, s.control = nil, nil
	s.mu.Unlock()
	if process != nil {
		process.stop()
	}
	if control != nil {
		_ = control.Close()
	}
}

type booting struct {
	root        func(slug string) string
	agent       func(AgentRequest) (AgentCommand, error)
	serve       func(state *sessionState, socket string) (io.Closer, error)
	shown       func(state *sessionState)
	teardown    func(state *sessionState)
	uncommitted func(sessionRoot string) ([]string, error)
	cleanup     func(sessionRoot string) error
	reveal      func(sessionRoot string) error
}

type Sessions struct {
	boot booting
	// showMu serialises the check and the boot, so a doubled rail click cannot
	// put two supervisors on one session.
	showMu sync.Mutex
	// touched wakes the chrome poller, so a switch reaches the page at once
	// rather than on the next tick.
	touched chan struct{}

	mu      sync.Mutex
	seq     int
	slugs   map[string]*sessionState
	terms   map[string]*sessionState
	showing *sessionState
	// history is the order sessions were shown in, most recent last, so a
	// supervisor exiting can hand the window back to the one before it.
	history []*sessionState
	// rail is the order the rail draws in, fixed by the first poll and only ever
	// appended to. A row's position addresses it from the keyboard, so a position
	// that moved would rename every shortcut under the user's fingers.
	rail []string

	now         func() time.Time
	retention   time.Duration
	agents      map[string]*agentActivity
	repoChanges map[string]struct{}
}

func newSessions() *Sessions {
	return newSessionsWithActivity(time.Now, finishedAgentRetention)
}

func newSessionsWithActivity(now func() time.Time, retention time.Duration) *Sessions {
	return &Sessions{
		touched:     make(chan struct{}, 1),
		slugs:       map[string]*sessionState{},
		terms:       map[string]*sessionState{},
		now:         now,
		retention:   retention,
		agents:      map[string]*agentActivity{},
		repoChanges: map[string]struct{}{},
	}
}

func (s *Sessions) Show(slug string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	state := s.bySlug(slug)
	if state == nil {
		root := s.boot.root(slug)
		if root == "" {
			return unknownSession(slug)
		}
		booted, err := s.start(root, "", true)
		if err != nil {
			return err
		}
		state = booted
	}
	s.reveal(state)
	return nil
}

func (s *Sessions) CycleSticker(slug string) (session.Sticker, error) {
	if slug == "" || slug == "." || slug == ".." || strings.ContainsAny(slug, `/\\`) {
		return "", unknownSession(slug)
	}
	root := s.boot.root(slug)
	if root == "" || !directSessionRoot(root, slug) {
		return "", unknownSession(slug)
	}
	var sticker session.Sticker
	err := session.UpdateManifest(root, func(m session.Manifest) (session.Manifest, error) {
		if m.Slug != slug {
			return m, unknownSession(slug)
		}
		m.Sticker = m.Sticker.Next()
		sticker = m.Sticker
		return m, nil
	})
	if err != nil {
		return "", err
	}
	s.touch()
	return sticker, nil
}

func directSessionRoot(root, slug string) bool {
	clean := filepath.Clean(root)
	if filepath.Base(clean) != slug {
		return false
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(clean)
	return err == nil && resolved == filepath.Join(parent, slug)
}

// Reveal shows a session's directory in the file manager. It takes no lock and
// wakes nothing: which session is on screen and what the rail draws are both
// unchanged by it.
func (s *Sessions) Reveal(slug string) error {
	root := s.boot.root(slug)
	if root == "" {
		return unknownSession(slug)
	}
	return s.boot.reveal(root)
}

// RevealPath shows one directory inside a session in the file manager. The path
// is the page's, so it is refused unless it resolves inside the session it names.
func (s *Sessions) RevealPath(slug, path string) error {
	root := s.boot.root(slug)
	if root == "" {
		return unknownSession(slug)
	}
	inside, err := within(root, path)
	if err != nil || !inside {
		return ErrPathOutsideSession
	}
	return s.boot.reveal(path)
}

// within reports whether path resolves inside root, with both cleaned so a
// climbing path cannot leave the session by spelling.
func within(root, path string) (bool, error) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false, err
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

// Uncommitted names the repositories a cleanup would take changes from. It is a
// git status per repository, so it answers a click and never a poll.
func (s *Sessions) Uncommitted(slug string) ([]string, error) {
	root := s.boot.root(slug)
	if root == "" {
		return nil, unknownSession(slug)
	}
	dirty, err := s.boot.uncommitted(root)
	if err != nil {
		return nil, err
	}
	if dirty == nil {
		// A nil slice marshals as JSON null, which reaches a .length on the page.
		return []string{}, nil
	}
	return dirty, nil
}

// Cleanup ends a session and removes it from disk: its worktrees and its
// directory go, and the mirrors, its branch inside them and its documents stay.
func (s *Sessions) Cleanup(slug string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	root := s.boot.root(slug)
	if root == "" {
		return unknownSession(slug)
	}
	// Retiring first is what stops a supervisor running on an unlinked directory
	// and what stops the window recorder writing the manifest back after removal.
	if state := s.bySlug(slug); state != nil {
		s.retire(state)
	} else if pid, alive := session.AgentAlive(root); alive {
		return agentAlreadyRunning(filepath.Base(root), pid)
	}
	if err := s.boot.cleanup(root); err != nil {
		return err
	}
	s.dropAgentActivity(slug)
	s.touch()
	return nil
}

// start brings up a session about to be shown, and only one: a terminal under
// display:none has no layout box, so its PTY would size from the page's defaults.
func (s *Sessions) start(root, runnerID string, resume bool) (*sessionState, error) {
	// A crashed workbench leaves agent.pid behind with nothing on the other end,
	// so a dead pid boots. A live one is an error rather than a state.
	if pid, alive := session.AgentAlive(root); alive {
		return nil, agentAlreadyRunning(filepath.Base(root), pid)
	}
	// A session records the agent it was assembled with, so every boot after the
	// first starts that one rather than whatever the workbench was launched with.
	if runnerID == "" {
		if m, err := session.Load(root); err == nil {
			runnerID = m.Runner
		}
	}
	socket, err := workbench.NewSocketPath()
	if err != nil {
		return nil, err
	}
	command, err := s.boot.agent(AgentRequest{
		SessionRoot: root, Socket: socket, RunnerID: runnerID, Resume: resume,
	})
	if err != nil {
		return nil, err
	}
	state := s.add(root, command.Argv, withTerminalEnv(command.Env))
	state.provider = command.RunnerID
	control, err := s.boot.serve(state, socket)
	if err != nil {
		s.forget(state)
		return nil, err
	}
	if !state.serve(control) {
		_ = control.Close()
		s.forget(state)
		return nil, ErrNoSession
	}
	return state, nil
}

// adopt puts a freshly assembled session on screen. The overlay has no PTY to
// hand over, so the session boots itself here with nothing to resume.
func (s *Sessions) adopt(root, runnerID string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	state := s.bySlug(slugFor(root))
	if state == nil {
		booted, err := s.start(root, runnerID, false)
		if err != nil {
			return err
		}
		state = booted
	}
	s.reveal(state)
	return nil
}

// retire ends one session without ending the app.
func (s *Sessions) retire(state *sessionState) {
	s.boot.teardown(state)
	state.stop()
	fallback, wasShown := s.forget(state)
	if !wasShown {
		s.touch()
		return
	}
	s.reveal(fallback)
}

func (s *Sessions) stopAll() {
	for _, state := range s.all() {
		state.stop()
	}
	s.mu.Lock()
	s.agents = map[string]*agentActivity{}
	s.mu.Unlock()
}

func (s *Sessions) add(root string, argv, env []string) *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	slug := slugFor(root)
	tracker := s.agents[slug]
	if tracker == nil {
		tracker = newAgentActivity(s.now, s.retention)
		s.agents[slug] = tracker
	}
	state := &sessionState{
		terminal: fmt.Sprintf(terminalIDFormat, s.seq),
		agents:   tracker,
		argv:     argv,
		env:      env,
	}
	state.named.Store(&identity{slug: slugFor(root), root: root})
	s.slugs[state.slug()] = state
	s.terms[state.terminal] = state
	return state
}

func (s *Sessions) agentActivitySnapshots() map[string]agentActivitySnapshot {
	s.mu.Lock()
	trackers := make(map[string]*agentActivity, len(s.agents))
	for slug, tracker := range s.agents {
		trackers[slug] = tracker
	}
	s.mu.Unlock()
	snapshots := make(map[string]agentActivitySnapshot, len(trackers))
	for slug, tracker := range trackers {
		snapshots[slug] = tracker.snapshot()
	}
	return snapshots
}

func (s *Sessions) earliestAgentExpiry() time.Time {
	s.mu.Lock()
	trackers := make([]*agentActivity, 0, len(s.agents))
	for _, tracker := range s.agents {
		trackers = append(trackers, tracker)
	}
	s.mu.Unlock()
	var earliest time.Time
	for _, tracker := range trackers {
		if expiry := tracker.earliestExpiry(); !expiry.IsZero() && (earliest.IsZero() || expiry.Before(earliest)) {
			earliest = expiry
		}
	}
	return earliest
}

func (s *Sessions) dropAgentActivity(slug string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, slug)
}

func (s *Sessions) forget(state *sessionState) (fallback *sessionState, wasShown bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.slugs, state.slug())
	delete(s.terms, state.terminal)
	kept := without(s.history, state)
	s.history = kept
	if s.showing != state {
		return nil, false
	}
	s.showing = nil
	if len(kept) > 0 {
		return kept[len(kept)-1], true
	}
	return nil, true
}

// reveal puts a session on screen. A nil session is the window with none, which
// is what the landing path and the last retirement leave behind.
func (s *Sessions) reveal(state *sessionState) {
	s.mu.Lock()
	if s.showing != state {
		s.showing = state
		if state != nil {
			s.history = append(without(s.history, state), state)
		}
	}
	s.mu.Unlock()
	if s.boot.shown != nil {
		s.boot.shown(state)
	}
	s.touch()
}

func (s *Sessions) current() *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.showing
}

func (s *Sessions) bySlug(slug string) *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.slugs[slug]
}

func (s *Sessions) byTerminal(id string) (*sessionState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.terms[id]
	return state, ok
}

func (s *Sessions) all() []*sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*sessionState, 0, len(s.terms))
	for _, state := range s.terms {
		out = append(out, state)
	}
	return out
}

func (s *Sessions) touch() {
	select {
	case s.touched <- struct{}{}:
	default:
	}
}

func (s *Sessions) repositoriesChanged(root string) {
	s.mu.Lock()
	s.repoChanges[root] = struct{}{}
	s.mu.Unlock()
	s.touch()
}

func (s *Sessions) takeRepositoryChanges() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := make([]string, 0, len(s.repoChanges))
	for root := range s.repoChanges {
		roots = append(roots, root)
	}
	s.repoChanges = map[string]struct{}{}
	return roots
}

// without is history with one session removed, filtered in place.
func without(history []*sessionState, state *sessionState) []*sessionState {
	kept := history[:0]
	for _, seen := range history {
		if seen != state {
			kept = append(kept, seen)
		}
	}
	return kept
}

// railOrder puts a poll's rows in the order the rail was first drawn in. A session
// assembled since then goes to the front, because it is the most recent and that
// is the rule the rest of the list follows; nothing else moves a row.
func (s *Sessions) railOrder(rows []status.SessionRow) []status.SessionRow {
	found := make(map[string]status.SessionRow, len(rows))
	for _, row := range rows {
		found[row.Slug] = row
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	placed := make(map[string]bool, len(s.rail))
	for _, slug := range s.rail {
		placed[slug] = true
	}
	// rows arrive most recent first, so prepending the batch whole keeps them in
	// that order rather than reversing it.
	var fresh []string
	for _, row := range rows {
		if !placed[row.Slug] {
			placed[row.Slug] = true
			fresh = append(fresh, row.Slug)
		}
	}
	if len(fresh) > 0 {
		s.rail = append(fresh, s.rail...)
	}
	out := make([]status.SessionRow, 0, len(rows))
	for _, slug := range s.rail {
		// A session gone from disk keeps its place in the sequence, so it
		// returns to the same position rather than landing at the end.
		if row, ok := found[slug]; ok {
			out = append(out, row)
		}
	}
	return out
}

// slugFor is a session's key: the name of its directory. The landing path has no
// directory yet.
func slugFor(root string) string {
	if root == "" {
		return ""
	}
	return filepath.Base(root)
}
