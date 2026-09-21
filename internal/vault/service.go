package vault

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
)

type ProfileStatus struct {
	Index       *IndexStatus `json:"index,omitempty"`
	Profile     string       `json:"profile"`
	State       string       `json:"state"`
	Documents   int          `json:"documents"`
	Invalid     int          `json:"invalid"`
	Unsupported int          `json:"unsupported"`
	Conflicts   int          `json:"conflicts"`
}
type Status struct {
	Enabled  bool            `json:"enabled"`
	State    string          `json:"state"`
	Mode     string          `json:"mode"`
	Scope    ReadScope       `json:"scope"`
	Profiles []ProfileStatus `json:"profiles"`
}
type Service struct {
	workers   map[string]*profileWorker
	closeOnce sync.Once
	profiles  []Profile
	mappings  map[string]string
}

func NewService(profiles []Profile, mappings map[string]string) (*Service, error) {
	if err := ValidateProfiles(profiles, mappings); err != nil {
		return nil, err
	}
	s := &Service{profiles: append([]Profile(nil), profiles...), mappings: map[string]string{}}
	for org, id := range mappings {
		s.mappings[org] = id
	}
	return s, nil
}
func (s *Service) Status(scope Scope) Status {
	status := Status{Enabled: len(s.profiles) > 0, State: StateDisabled, Mode: ModeUncalibrated, Profiles: []ProfileStatus{}}
	if !status.Enabled {
		return status
	}
	if err := ValidateProfiles(s.profiles, s.mappings); err != nil {
		status.State = StateUnavailable
		return status
	}
	status.State = StateInitializing
	if len(s.workers) > 0 {
		status.State = StateReady
	}
	resolved, err := ResolveReadProfiles(s.profiles, s.mappings, scope)
	status.Scope = resolved
	if err != nil {
		status.State = StateUnavailable
		return status
	}
	if resolved.SelectionRequired || len(resolved.Unmapped) > 0 {
		status.State = StateIncomplete
	}
	for _, profile := range s.profiles {
		if !slices.Contains(resolved.Profiles, profile.ID) {
			continue
		}
		inv, err := scan(profile)
		if worker := s.workers[profile.ID]; worker != nil {
			inv = worker.applyIdentityConflicts(inv)
		}
		ps := ProfileStatus{Profile: profile.ID, State: StateInitializing, Invalid: inv.invalid, Unsupported: inv.unsupported, Conflicts: len(inv.conflicts)}
		for _, entry := range inv.entries {
			if !inv.conflicts[entry.Document.ID] && entry.Supported {
				ps.Documents++
			}
		}
		if err != nil {
			ps.State = StateUnavailable
			status.State = StateUnavailable
		} else if ps.Invalid+ps.Unsupported+ps.Conflicts > 0 {
			ps.State = StateIncomplete
			if status.State != StateUnavailable {
				status.State = StateIncomplete
			}
		}
		if worker := s.workers[profile.ID]; worker != nil {
			index := worker.snapshot(inv)
			ps.Index = &index
			switch index.State {
			case StateUnavailable:
				ps.State = StateUnavailable
				status.State = StateUnavailable
			case StateReady:
				if ps.State == StateInitializing {
					ps.State = StateReady
				}
			case StateIncomplete:
				ps.State = StateIncomplete
				if status.State != StateUnavailable {
					status.State = StateIncomplete
				}
			default:
				if ps.State == StateInitializing && status.State == StateReady {
					status.State = StateInitializing
				}
			}
		}
		status.Profiles = append(status.Profiles, ps)
	}
	return status
}
func (s *Service) Read(scope Scope, ref Reference) (ReadResult, error) { return s.Resolve(scope, ref) }
func (s *Service) Resolve(scope Scope, ref Reference) (ReadResult, error) {
	if err := ValidateProfiles(s.profiles, s.mappings); err != nil {
		return ReadResult{}, err
	}
	resolved, err := ResolveReadProfiles(s.profiles, s.mappings, scope)
	if err != nil {
		return ReadResult{}, err
	}
	if ref.Profile != "" && !slices.Contains(resolved.Profiles, ref.Profile) {
		return ReadResult{}, ErrScope
	}
	if ref.ID == "" && (ref.Path == "" || ref.Profile == "") {
		return ReadResult{}, ErrNotFound
	}
	var found *ReadResult
	var origin Profile
	for _, profile := range s.profiles {
		if !slices.Contains(resolved.Profiles, profile.ID) || (ref.Profile != "" && profile.ID != ref.Profile) {
			continue
		}
		inv, err := scan(profile)
		if err != nil {
			return ReadResult{}, err
		}
		if ref.ID != "" && inv.conflicts[ref.ID] {
			return ReadResult{}, ErrConflict
		}
		for _, entry := range inv.entries {
			if ref.ID != "" && entry.Document.ID != ref.ID {
				continue
			}
			if ref.Path != "" && entry.Reference.Path != ref.Path {
				continue
			}
			if inv.conflicts[entry.Document.ID] {
				return ReadResult{}, ErrConflict
			}
			if found != nil {
				if found.Reference.Profile != entry.Reference.Profile {
					return ReadResult{}, ErrAmbiguous
				}
				continue
			}
			copy := entry
			found = &copy
			origin = profile
		}
	}
	if found == nil {
		return ReadResult{}, ErrNotFound
	}
	if worker := s.workers[origin.ID]; worker != nil {
		if err := worker.checkIdentity(*found); err != nil {
			return ReadResult{}, err
		}
	}
	return resolveFile(origin, *found)
}
func NewIndexedService(profiles []Profile, mappings map[string]string, options IndexOptions) (*Service, error) {
	s, err := NewService(profiles, mappings)
	if err != nil {
		return nil, err
	}
	s.workers = map[string]*profileWorker{}
	for _, profile := range s.profiles {
		directory := profileStatePath(options.StateDir, profile)
		ledger, ledgerErr := loadLedger(directory)
		canonical := canonicalProfileRoot(profile.Root)
		ctx, cancel := context.WithCancel(context.Background())
		worker := &profileWorker{profile: profile, canonicalRoot: canonical, stateDir: directory, options: options, validate: func() error { return ValidateProfiles(s.profiles, s.mappings) }, ledger: ledger, ledgerErr: ledgerErr, allowed: map[string]string{}, lineage: map[string]LineageState{}, status: IndexStatus{State: StateInitializing}, retry: make(chan struct{}, 1), cancel: cancel, done: make(chan struct{})}
		s.workers[profile.ID] = worker
		if options.Failure != "" || !filepath.IsAbs(options.StateDir) || options.Provider == nil || !componentPattern.MatchString(options.InstallationID) {
			worker.status.State = StateUnavailable
			worker.status.Error = ErrIndexState.Error()
			if options.Failure != "" {
				worker.status.Error = options.Failure
			}
			close(worker.done)
			continue
		}
		go worker.run(ctx)
	}
	return s, nil
}
func (s *Service) Retry(profile string) error {
	worker := s.workers[profile]
	if worker == nil {
		return ErrScope
	}
	worker.signal()
	return nil
}
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		for _, worker := range s.workers {
			worker.cancel()
		}
		for _, worker := range s.workers {
			<-worker.done
		}
	})
	return nil
}
