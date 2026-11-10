package core

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type SessionState struct {
	// terminal addresses the conversation PTY rather than the session, because
	// the page attaches its stream before the supervisor is running.
	terminal    string
	agents      *agentActivity
	provider    string
	argv        []string
	env         []string
	sessionSlug string
	sessionRoot string
	// control is the session's own listener, and nil for a session whose control
	// arrives on the process socket instead.
	control io.Closer
	output  *outputStream

	mu              sync.Mutex
	stopped         bool
	windowsClosed   atomic.Bool
	bugReports      map[string]*bugReportRecord
	latestBugReport string
	process         *PTY
	// exit is nil until the supervisor exits of its own accord.
	exit           *int
	overviewWindow string
	// picker is the escalation waiting for the user to arrive at this session.
	// Agent requests expire with their caller; one opened by the user does not.
	picker *workbench.PickerRequest
	// shells counts the shells the session has had rather than the ones still
	// open: a number freed by a close would name two terminals at once.
	shells int
}

// RequestPicker queues an escalation on this session. A later request replaces
// an earlier one: both pollers then read the one stanza the confirm writes.
func (s *SessionState) RequestPicker(req workbench.PickerRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.picker = &req
}

func (s *SessionState) PendingPicker() *workbench.PickerRequest {
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

// ClearPicker is confirm and cancel both: arriving at the session again must not
// redraw a picker that has been answered.
func (s *SessionState) ClearPicker() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.picker = nil
}

// Slug and Root tolerate a nil session, which is the window showing none. Both
// are fixed at construction, so neither takes the lock.
func (s *SessionState) Slug() string {
	if s == nil {
		return ""
	}
	return s.sessionSlug
}

func (s *SessionState) Root() string {
	if s == nil {
		return ""
	}
	return s.sessionRoot
}

// alive is whether the session still has a supervisor behind it. A session
// between add and the page's first start has no PTY yet and counts as alive:
// the page needs its terminal id to make that call.
func (s *SessionState) alive() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.stopped && (s.process == nil || !s.process.Done())
}

func (s *SessionState) admissionClosed() bool {
	if s == nil {
		return false
	}
	return s.windowsClosed.Load()
}

// attach launches the supervisor under a PTY sized to the terminal displaying
// it. The page attaches on every load, so a reload replays the conversation
// rather than forking a second agent.
func (s *SessionState) attach(events Events, exited func(*SessionState, int), size *Size, from *uint64, deliver func(Output)) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	if s.process == nil {
		defer s.mu.Unlock()
		if size == nil {
			return ErrTerminalNotStarted
		}
		return s.startLocked(events, exited, *size)
	}
	s.mu.Unlock()
	return s.output.attach(from, func(out Output) error {
		s.mu.Lock()
		process := s.process
		if s.exit != nil {
			code := *s.exit
			out.ExitCode = &code
		}
		s.mu.Unlock()
		if size != nil && out.ExitCode == nil && process != nil && !process.Done() {
			if err := process.Resize(size.Cols, size.Rows); err != nil {
				return err
			}
		}
		deliver(out)
		return nil
	})
}

func (s *SessionState) startLocked(events Events, exited func(*SessionState, int), size Size) error {
	process, err := StartPTY(s.argv, s.env, s.Root(), size.Cols, size.Rows)
	if err != nil {
		return err
	}
	s.process = process
	go process.Pump(
		func(b []byte) {
			s.agents.output()
			s.output.publish(b, func(out Output) { events.TerminalOutput(TerminalConversation, s.terminal, out) })
		},
		func(code int) { s.childExited(events, exited, code) },
	)
	return nil
}

func (s *SessionState) childExited(events Events, exited func(*SessionState, int), code int) {
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return
	}
	s.recordExit(code)
	s.output.ordered(func() {
		s.mu.Lock()
		s.exit = &code
		s.mu.Unlock()
		events.TerminalExit(TerminalConversation, s.terminal, code)
	})
	if exited != nil {
		exited(s, code)
	}
}

// recordExit logs a death the workbench did not cause. A supervisor it stopped
// itself has nothing to explain, and a line plus a tail for every reload and
// every quit would rotate the one crash that mattered off the disk.
func (s *SessionState) recordExit(code int) {
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return
	}
	var last string
	if code != 0 {
		last = s.output.ring.Text(false)
	}
	RecordAgentExit(s.Root(), s.provider, code, last)
}

// terminalReplies matches writes xterm.js makes on its own: focus and mouse
// reports, and answers to the runner's status, attribute, window and colour queries.
var terminalReplies = regexp.MustCompile(`^(?:\x1b\[[IO]|\x1b\[<\d+;\d+;\d+[Mm]|\x1b\[M...|\x1b\[\??\d+;\d+R|\x1b\[\d*n|\x1b\[[?>][\d;]*c|\x1b\[\??[\d;]*\$y|\x1b\[[\d;]*t|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1bP[^\x1b]*\x1b\\)+$`)

func (s *SessionState) write(data []byte) error {
	if !terminalReplies.Match(data) {
		s.agents.input()
	}
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	if process == nil {
		return ErrTerminalNotStarted
	}
	return process.Write(data)
}

func (s *SessionState) resize(cols, rows int) error {
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	if process == nil {
		return nil
	}
	return process.Resize(cols, rows)
}

// serve records the listener the session answers on, refusing once it has been
// stopped: a listener installed after teardown is one nobody will ever close.
func (s *SessionState) serve(control io.Closer) bool {
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
func (s *SessionState) stop() {
	s.mu.Lock()
	s.stopped = true
	s.pruneBugReports(time.Now())
	process, control := s.process, s.control
	s.process, s.control = nil, nil
	s.mu.Unlock()
	if process != nil {
		process.Stop()
	}
	if control != nil {
		_ = control.Close()
	}
}

type booting struct {
	root        func(slug string) string
	agent       func(AgentRequest) (AgentCommand, error)
	serve       func(state *SessionState, socket string) (io.Closer, error)
	shown       func(state *SessionState)
	teardown    func(state *SessionState)
	uncommitted func(sessionRoot string) ([]string, error)
	cleanup     func(sessionRoot string) error
	reveal      func(sessionRoot string) error
	copyImage   func(path string) error
	copyFile    func(path string) error
	stickers    func() []session.Sticker
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
	slugs   map[string]*SessionState
	terms   map[string]*SessionState
	showing *SessionState
	// history is the order sessions were shown in, most recent last, so a
	// supervisor exiting can hand the window back to the one before it.
	history []*SessionState
	// rail retains user moves and missing slugs; new sessions prepend on polling.
	rail []string

	now    func() time.Time
	agents map[string]*agentActivity
	// trees pushes the shown session's agent tree; nil until the workbench runs.
	trees       *agentTrees
	repoChanges map[string]struct{}
	pullChanges map[string]struct{}

	placementShown atomic.Bool
}

func newSessions() *Sessions {
	return newSessionsWithActivity(time.Now)
}

func newSessionsWithActivity(now func() time.Time) *Sessions {
	return &Sessions{
		touched:     make(chan struct{}, 1),
		slugs:       map[string]*SessionState{},
		terms:       map[string]*SessionState{},
		now:         now,
		agents:      map[string]*agentActivity{},
		repoChanges: map[string]struct{}{},
		pullChanges: map[string]struct{}{},
	}
}

func (s *Sessions) Show(slug string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	state := s.BySlug(slug)
	if state.alive() {
		s.reveal(state)
		return nil
	}
	// A supervisor that died left its state registered, and a pane drawn against
	// it reaches nothing.
	var fallback *SessionState
	var emptied bool
	if state != nil {
		fallback, emptied = s.recycle(state)
	}
	root := s.boot.root(slug)
	if root == "" {
		if emptied {
			s.reveal(fallback)
		}
		return UnknownSession(slug)
	}
	return s.replace(root, fallback, emptied)
}

// Reload restarts a session's supervisor and resumes the conversation, which is
// the way out of one that is wedged rather than dead. A supervisor left behind
// by a previous workbench is refused rather than killed.
func (s *Sessions) Reload(slug string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	root := s.boot.root(slug)
	if root == "" {
		return UnknownSession(slug)
	}
	var fallback *SessionState
	var emptied bool
	if state := s.BySlug(slug); state != nil {
		fallback, emptied = s.recycle(state)
	}
	return s.replace(root, fallback, emptied)
}

func (s *Sessions) Unload(slug string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	root, err := s.validatedRoot(slug)
	if err != nil {
		return err
	}
	manifest, err := session.Load(root)
	if err != nil || manifest.Slug != slug {
		return UnknownSession(slug)
	}
	state := s.BySlug(slug)
	if state == nil {
		return nil
	}
	fallback, wasShown := s.recycle(state)
	s.dropAgentActivity(slug)
	if wasShown {
		s.reveal(fallback)
	} else {
		s.Touch()
	}
	return nil
}

func validateSessionSlug(slug string) error {
	if slug == "" || slug == "." || slug == ".." || strings.ContainsAny(slug, `/\`) {
		return UnknownSession(slug)
	}
	return nil
}

func (s *Sessions) validatedRoot(slug string) (string, error) {
	if err := validateSessionSlug(slug); err != nil {
		return "", err
	}
	root := s.boot.root(slug)
	if root == "" || !directSessionRoot(root, slug) {
		return "", UnknownSession(slug)
	}
	return root, nil
}

func (s *Sessions) Reorder(slug, targetSlug string) error {
	if err := validateSessionSlug(slug); err != nil {
		return err
	}
	if err := validateSessionSlug(targetSlug); err != nil {
		return err
	}
	s.mu.Lock()
	from, to := slices.Index(s.rail, slug), slices.Index(s.rail, targetSlug)
	if from < 0 || to < 0 {
		s.mu.Unlock()
		if from < 0 {
			return UnknownSession(slug)
		}
		return UnknownSession(targetSlug)
	}
	if from == to {
		s.mu.Unlock()
		return nil
	}
	if from < to {
		copy(s.rail[from:to], s.rail[from+1:to+1])
	} else {
		copy(s.rail[to+1:from+1], s.rail[to:from])
	}
	s.rail[to] = slug
	s.mu.Unlock()
	s.Touch()
	return nil
}

// replace boots a session and puts it on screen. A boot that fails after
// recycling had emptied the window hands the window back rather than leaving it
// on nothing.
func (s *Sessions) replace(root string, fallback *SessionState, emptied bool) error {
	booted, err := s.start(root, "", true)
	if err != nil {
		if emptied {
			s.reveal(fallback)
		}
		return err
	}
	s.reveal(booted)
	return nil
}

func (s *Sessions) CycleSticker(slug string) (session.Sticker, error) {
	root, err := s.validatedRoot(slug)
	if err != nil {
		return "", err
	}
	var order []session.Sticker
	if s.boot.stickers != nil {
		order = s.boot.stickers()
	}
	var sticker session.Sticker
	err = session.UpdateManifest(root, func(m session.Manifest) (session.Manifest, error) {
		if m.Slug != slug {
			return m, UnknownSession(slug)
		}
		m.Sticker = m.Sticker.Next(order)
		sticker = m.Sticker
		return m, nil
	})
	if err != nil {
		return "", err
	}
	s.Touch()
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
		return UnknownSession(slug)
	}
	return s.boot.reveal(root)
}

// RevealPath shows one directory inside a session in the file manager. The path
// is the page's, so it is refused unless it resolves inside the session it names.
func (s *Sessions) RevealPath(slug, path string) error {
	root := s.boot.root(slug)
	if root == "" {
		return UnknownSession(slug)
	}
	if _, inside := sessionpaths.Within(root, path); !inside {
		return ErrPathOutsideSession
	}
	return s.boot.reveal(path)
}

// CopyAsset puts one of a gallery's assets on the clipboard: an image as a
// picture, a video as a file. The page's path is refused unless it resolves
// inside the session, and the resolved target decides the kind.
func (s *Sessions) CopyAsset(slug, path string) error {
	root := s.boot.root(slug)
	if root == "" {
		return UnknownSession(slug)
	}
	target, err := launch.ResolveSessionFile(root, path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPathOutsideSession, err)
	}
	media, ok := workbench.AssetMedia(target)
	if !ok {
		return ErrNotAnAsset
	}
	if media.Kind == workbench.MediaVideo {
		return s.boot.copyFile(target)
	}
	return s.boot.copyImage(target)
}

// Uncommitted names the repositories a cleanup would take changes from. It is a
// git status per repository, so it answers a click and never a poll.
func (s *Sessions) Uncommitted(slug string) ([]string, error) {
	root := s.boot.root(slug)
	if root == "" {
		return nil, UnknownSession(slug)
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
		return UnknownSession(slug)
	}
	// Retiring first is what stops a supervisor running on an unlinked directory
	// and what stops the window recorder writing the manifest back after removal.
	if state := s.BySlug(slug); state != nil {
		s.retireLocked(state)
	} else if pid, alive := session.AgentAlive(root); alive {
		return agentAlreadyRunning(filepath.Base(root), pid)
	}
	if err := s.boot.cleanup(root); err != nil {
		return err
	}
	s.dropAgentActivity(slug)
	s.Touch()
	return nil
}

// start brings up a session about to be shown, and only one: a terminal under
// display:none has no layout box, so its PTY would size from the page's defaults.
func (s *Sessions) start(root, runnerID string, resume bool) (*SessionState, error) {
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
	state := s.add(root, command.Argv, WithTerminalEnv(command.Env))
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

// Adopt puts a freshly assembled session on screen. The overlay has no PTY to
// hand over, so the session boots itself here with nothing to resume.
func (s *Sessions) Adopt(root, runnerID string) error {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	state := s.BySlug(slugFor(root))
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

// recycle drops a session without putting anything in its place, so the window
// never falls back to the session before this one on its way to a replacement.
// The caller reveals under the same showMu: the replacement it booted, or the
// fallback returned here when that boot failed.
func (s *Sessions) recycle(state *SessionState) (fallback *SessionState, wasShown bool) {
	s.boot.teardown(state)
	state.stop()
	return s.forget(state)
}

// retire ends one session without ending the app.
func (s *Sessions) retire(state *SessionState) {
	s.showMu.Lock()
	defer s.showMu.Unlock()
	state.mu.Lock()
	stopped := state.stopped
	state.mu.Unlock()
	if stopped || s.BySlug(state.Slug()) != state {
		return
	}
	s.retireLocked(state)
}

func (s *Sessions) retireLocked(state *SessionState) {
	s.boot.teardown(state)
	state.stop()
	fallback, wasShown := s.forget(state)
	if !wasShown {
		s.Touch()
		return
	}
	if fallback == nil {
		if root := s.nextInactiveRoot(state.Slug()); root != "" {
			fallback, _ = s.start(root, "", true)
		}
	}
	s.reveal(fallback)
}

func (s *Sessions) nextInactiveRoot(after string) string {
	s.mu.Lock()
	rail := append([]string(nil), s.rail...)
	registered := make(map[string]bool, len(s.slugs))
	for slug := range s.slugs {
		registered[slug] = true
	}
	s.mu.Unlock()

	start := 0
	for i, slug := range rail {
		if slug == after {
			start = (i + 1) % len(rail)
			break
		}
	}
	for i := range rail {
		slug := rail[(start+i)%len(rail)]
		if slug == after || registered[slug] {
			continue
		}
		if root := s.boot.root(slug); root != "" {
			return root
		}
	}
	return ""
}

func (s *Sessions) stopAll() {
	for _, state := range s.all() {
		state.stop()
	}
	s.mu.Lock()
	s.agents = map[string]*agentActivity{}
	s.mu.Unlock()
}

func (s *Sessions) add(root string, argv, env []string) *SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	slug := slugFor(root)
	tracker := s.agents[slug]
	if tracker == nil {
		tracker = newAgentActivity(s.now)
		s.agents[slug] = tracker
	}
	state := &SessionState{
		terminal:    fmt.Sprintf(terminalIDFormat, s.seq),
		agents:      tracker,
		argv:        argv,
		env:         env,
		sessionSlug: slug,
		sessionRoot: root,
		output:      newOutputStream(WindowScrollback),
	}
	s.slugs[state.Slug()] = state
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

func (s *Sessions) dropAgentActivity(slug string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, slug)
}

func (s *Sessions) forget(state *SessionState) (fallback *SessionState, wasShown bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.slugs, state.Slug())
	delete(s.terms, state.terminal)
	kept := without(s.history, state)
	s.history = kept
	if s.showing != state {
		return nil, false
	}
	s.showing = nil
	for i := len(kept) - 1; i >= 0; i-- {
		seen := kept[i]
		if s.slugs[seen.Slug()] == seen && seen.alive() {
			return seen, true
		}
	}
	return nil, true
}

// reveal puts a session on screen. A nil session is the window with none, which
// is what the landing path and the last retirement leave behind.
func (s *Sessions) reveal(state *SessionState) {
	s.mu.Lock()
	if s.showing != state {
		s.showing = state
		if state != nil {
			s.history = append(without(s.history, state), state)
		}
	}
	s.mu.Unlock()
	if state != nil {
		state.agents.seen()
	}
	if s.boot.shown != nil {
		s.boot.shown(state)
	}
	s.Touch()
}

func (s *Sessions) Current() *SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.showing
}

func (s *Sessions) BySlug(slug string) *SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.slugs[slug]
}

func (s *Sessions) ByTerminal(id string) (*SessionState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.terms[id]
	return state, ok
}

func (s *Sessions) all() []*SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*SessionState, 0, len(s.terms))
	for _, state := range s.terms {
		out = append(out, state)
	}
	return out
}

func (s *Sessions) Placing() bool { return s.placementShown.Load() }

func (s *Sessions) SetPlacing(shown bool) {
	s.placementShown.Store(shown)
	s.Touch()
}

func (s *Sessions) Touch() {
	select {
	case s.touched <- struct{}{}:
	default:
	}
}

func (s *Sessions) RepositoriesChanged(root string) { s.changed(&s.repoChanges, root) }

func (s *Sessions) takeRepositoryChanges() []string { return s.take(&s.repoChanges) }

// pullRequestsChanged marks a session's pull requests stale without touching its repo measurements.
func (s *Sessions) pullRequestsChanged(root string) { s.changed(&s.pullChanges, root) }

func (s *Sessions) takePullRequestChanges() []string { return s.take(&s.pullChanges) }

func (s *Sessions) changed(set *map[string]struct{}, root string) {
	s.mu.Lock()
	(*set)[root] = struct{}{}
	s.mu.Unlock()
	s.Touch()
}

// QueuePicker records an agent escalation on the session it names and raises
// nothing. The overlay opens when the user next arrives there, so a request they
// never arrive at expires unseen rather than taking the screen.
func (s *Sessions) QueuePicker(req workbench.PickerRequest) error {
	slug := slugFor(req.SessionRoot)
	state := s.BySlug(slug)
	if state == nil {
		return UnknownSession(slug)
	}
	state.RequestPicker(req)
	s.Touch()
	return nil
}

func (s *Sessions) take(set *map[string]struct{}) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := make([]string, 0, len(*set))
	for root := range *set {
		roots = append(roots, root)
	}
	*set = map[string]struct{}{}
	return roots
}

// without is history with one session removed, filtered in place.
func without(history []*SessionState, state *SessionState) []*SessionState {
	kept := history[:0]
	for _, seen := range history {
		if seen != state {
			kept = append(kept, seen)
		}
	}
	clear(history[len(kept):])
	return kept
}

// railOrder preserves user moves and prepends newly discovered sessions.
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

// slugFor is a session's key: the name of its directory.
func slugFor(root string) string {
	if root == "" {
		return ""
	}
	return filepath.Base(root)
}
