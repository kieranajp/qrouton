package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// ProfilesView lists each installed runner that has profiles. SessionShown is
// whether Sign in has a pane to open its tab in.
type ProfilesView struct {
	Runners      []RunnerProfiles `json:"runners"`
	SessionShown bool             `json:"sessionShown"`
}

// RunnerProfiles lists the named profiles only; every runner also has its default.
type RunnerProfiles struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Credentials []string     `json:"credentials"`
	Warning     string       `json:"warning,omitempty"`
	Profiles    []ProfileRow `json:"profiles"`
}

// ProfileRow counts the sessions whose manifest names this profile. Uncounted
// means the sessions root could not be read, so Sessions is not a count.
type ProfileRow struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Dir       string `json:"dir"`
	Sessions  int    `json:"sessions"`
	Uncounted bool   `json:"uncounted,omitempty"`
}

type ProfileInput struct {
	Runner string `json:"runner"`
	Label  string `json:"label"`
	Dir    string `json:"dir"`
}

type ProfileSuggestion struct {
	ID  string `json:"id"`
	Dir string `json:"dir"`
}

// ProfileStatus is one probe; an empty Profile is the runner's default.
type ProfileStatus struct {
	Runner  string             `json:"runner"`
	Profile string             `json:"profile"`
	State   launch.SignInState `json:"state"`
	Email   string             `json:"email,omitempty"`
}

// Profiles edits the runner account profiles. Every edit saves at once, since
// Sign in needs the profile's folder before the form is saved.
type Profiles struct {
	cfg          *config.Config
	pinnedRoot   string
	reconfigure  func(*config.Config) error
	runners      func() ([]assembly.Runner, error)
	launcher     Launcher
	sessions     *Sessions
	windows      *Windows
	environ      func() []string
	probeTimeout time.Duration
}

func newProfiles(cfg *config.Config, reconfigure func(*config.Config) error, runners func() ([]assembly.Runner, error),
	launcher Launcher, sessions *Sessions, windows *Windows) *Profiles {
	return &Profiles{
		cfg:          cfg,
		pinnedRoot:   pinnedFileRoot(),
		reconfigure:  reconfigure,
		runners:      runners,
		launcher:     launcher,
		sessions:     sessions,
		windows:      windows,
		environ:      os.Environ,
		probeTimeout: profileProbeTimeout,
	}
}

func (p *Profiles) View() (ProfilesView, error) {
	runners, err := p.profiled()
	if err != nil {
		return ProfilesView{}, err
	}
	cfg := p.cfg.Snapshot()
	manifests, scanErr := session.Scan(cfg.Root)
	view := ProfilesView{Runners: []RunnerProfiles{}, SessionShown: p.sessions.Current() != nil}
	for _, r := range runners {
		group := RunnerProfiles{ID: r.ID, Label: r.Label, Credentials: launch.CredentialsSet(r.ID, p.environ()),
			Profiles: []ProfileRow{}}
		if len(group.Credentials) > 0 {
			group.Warning = fmt.Sprintf(credentialWarningFormat, strings.Join(group.Credentials, credentialJoin), r.Label)
		}
		for _, profile := range cfg.Profiles[r.ID] {
			group.Profiles = append(group.Profiles, ProfileRow{ID: profile.ID, Label: profile.Label, Dir: profile.Dir,
				Sessions: pinnedSessions(manifests, r.ID, profile.ID), Uncounted: scanErr != nil})
		}
		view.Runners = append(view.Runners, group)
	}
	return view, nil
}

func pinnedSessions(manifests []session.Manifest, runner, profile string) int {
	n := 0
	for _, m := range manifests {
		if m.Runner == runner && m.Profile == profile {
			n++
		}
	}
	return n
}

func (p *Profiles) profiled() ([]assembly.Runner, error) {
	if p.runners == nil {
		return nil, nil
	}
	runners, err := p.runners()
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(runners, func(r assembly.Runner) bool {
		return !r.Installed || !launch.SupportsProfiles(r.ID)
	}), nil
}

func (p *Profiles) Suggest(runner, label string) ProfileSuggestion {
	id := session.Slugify(label)
	if id == "" {
		return ProfileSuggestion{}
	}
	return ProfileSuggestion{ID: id, Dir: launch.DefaultProfileDir(runner, id)}
}

// Add derives the id from the label, fixed for the profile's life, and creates
// its folder before saving.
func (p *Profiles) Add(in ProfileInput) (ProfilesView, error) {
	if !launch.SupportsProfiles(in.Runner) {
		return ProfilesView{}, fmt.Errorf("%w: %q", launch.ErrProfilesUnsupported, in.Runner)
	}
	label := strings.TrimSpace(in.Label)
	id := session.Slugify(label)
	if id == "" {
		return ProfilesView{}, ErrProfileLabel
	}
	if _, taken := p.cfg.Profile(in.Runner, id); taken {
		return ProfilesView{}, fmt.Errorf(profileTakenFormat, ErrProfileTaken, id)
	}
	dir := strings.TrimSpace(in.Dir)
	if dir == "" {
		dir = launch.DefaultProfileDir(in.Runner, id)
	}
	dir = config.ExpandHome(dir)
	if !filepath.IsAbs(dir) {
		return ProfilesView{}, fmt.Errorf(profileFolderFormat, ErrProfileFolder, in.Dir)
	}
	dir = filepath.Clean(dir)
	if err := launch.PrepareProfileDir(dir); err != nil {
		return ProfilesView{}, err
	}
	profile := config.RunnerProfile{ID: id, Label: label, Dir: dir}
	return p.save(func(profiles []config.RunnerProfile) []config.RunnerProfile {
		if indexProfile(profiles, id) >= 0 {
			return profiles
		}
		return append(profiles, profile)
	}, in.Runner)
}

// Rename changes the label only: the id and the folder stay, so pinned
// sessions and the stored login do too.
func (p *Profiles) Rename(runner, id, label string) (ProfilesView, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return ProfilesView{}, ErrProfileLabel
	}
	if _, ok := p.cfg.Profile(runner, id); !ok {
		return ProfilesView{}, missingProfile(runner, id)
	}
	return p.save(func(profiles []config.RunnerProfile) []config.RunnerProfile {
		if at := indexProfile(profiles, id); at >= 0 {
			profiles[at].Label = label
		}
		return profiles
	}, runner)
}

// Delete forgets the profile and leaves its folder, which holds the login and
// the history of every session pinned to it.
func (p *Profiles) Delete(runner, id string) (ProfilesView, error) {
	if _, ok := p.cfg.Profile(runner, id); !ok {
		return ProfilesView{}, missingProfile(runner, id)
	}
	return p.save(func(profiles []config.RunnerProfile) []config.RunnerProfile {
		if at := indexProfile(profiles, id); at >= 0 {
			return slices.Delete(profiles, at, at+1)
		}
		return profiles
	}, runner)
}

func indexProfile(profiles []config.RunnerProfile, id string) int {
	return slices.IndexFunc(profiles, func(existing config.RunnerProfile) bool { return existing.ID == id })
}

func missingProfile(runner, id string) error {
	return fmt.Errorf(profileMissingFormat, launch.ErrProfileUnavailable, runner, id)
}

// save applies edit to the runner's profiles in config.json as written and in
// the running config alike.
func (p *Profiles) save(edit func([]config.RunnerProfile) []config.RunnerProfile, runner string) (ProfilesView, error) {
	err := saveConfig(p.cfg, p.pinnedRoot, func(next, live *config.Config) {
		for _, c := range []*config.Config{next, live} {
			profiles := edit(slices.Clone(c.Profiles[runner]))
			if c.Profiles == nil {
				c.Profiles = map[string][]config.RunnerProfile{}
			}
			c.Profiles[runner] = profiles
			if len(profiles) == 0 {
				delete(c.Profiles, runner)
			}
			if len(c.Profiles) == 0 {
				c.Profiles = nil
			}
		}
	}, p.reconfigure, nil)
	if err != nil {
		return ProfilesView{}, err
	}
	return p.View()
}

// Status probes every profile of every listed runner at once, the defaults
// included. A probe still running at the deadline reads Unknown.
func (p *Profiles) Status() ([]ProfileStatus, error) {
	runners, err := p.profiled()
	if err != nil {
		return nil, err
	}
	cfg := p.cfg.Snapshot()
	var probes []ProfileStatus
	var profiles []config.RunnerProfile
	for _, r := range runners {
		probes = append(probes, ProfileStatus{Runner: r.ID, State: launch.SignInUnknown})
		profiles = append(profiles, config.RunnerProfile{})
		for _, profile := range cfg.Profiles[r.ID] {
			probes = append(probes, ProfileStatus{Runner: r.ID, Profile: profile.ID, State: launch.SignInUnknown})
			profiles = append(profiles, profile)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.probeTimeout)
	defer cancel()
	type probed struct {
		at     int
		signIn launch.SignIn
	}
	results := make(chan probed, len(probes))
	for i := range probes {
		go func() { results <- probed{i, p.launcher.ProbeSignIn(ctx, probes[i].Runner, profiles[i])} }()
	}
	for range probes {
		select {
		case r := <-results:
			probes[r.at].State, probes[r.at].Email = r.signIn.State, r.signIn.Email
		case <-ctx.Done():
			return probes, nil
		}
	}
	return probes, nil
}

// SignIn opens the runner's login in a tab of the session on screen and
// answers the tab's id, whose exit says the login has finished.
func (p *Profiles) SignIn(runner, id string) (string, error) {
	owner := p.sessions.Current()
	if owner == nil || owner.Root() == "" {
		return "", ErrNoSessionShown
	}
	profile := config.RunnerProfile{}
	label := fmt.Sprintf(signInLabelFormat, launch.RunnerLabel(runner))
	if id != "" {
		var ok bool
		if profile, ok = p.cfg.Profile(runner, id); !ok {
			return "", missingProfile(runner, id)
		}
		if err := launch.PrepareProfileDir(profile.Dir); err != nil {
			return "", err
		}
		label = fmt.Sprintf(signInProfileLabelFormat, launch.RunnerLabel(runner), profile.Label)
	}
	if p.launcher == nil {
		return "", fmt.Errorf("%w: %q", launch.ErrRunnerUnavailable, runner)
	}
	argv, err := p.launcher.SignInCommand(runner, profile)
	if err != nil {
		return "", err
	}
	return p.windows.openAgentWindow(owner, workbench.WindowOptions{
		Kind:    workbench.KindTerminal,
		Label:   label,
		Icon:    workbench.IconCommand,
		Cwd:     owner.Root(),
		Command: argv,
		Select:  true,
	})
}
