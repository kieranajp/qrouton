package vault

import "slices"

type ProfileStatus struct {
	Profile     string `json:"profile"`
	State       string `json:"state"`
	Documents   int    `json:"documents"`
	Invalid     int    `json:"invalid"`
	Unsupported int    `json:"unsupported"`
	Conflicts   int    `json:"conflicts"`
}
type Status struct {
	Enabled  bool            `json:"enabled"`
	State    string          `json:"state"`
	Mode     string          `json:"mode"`
	Scope    ReadScope       `json:"scope"`
	Profiles []ProfileStatus `json:"profiles"`
}
type Service struct {
	profiles []Profile
	mappings map[string]string
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
	return resolveFile(origin, *found)
}
func (s *Service) Close() error { return nil }
