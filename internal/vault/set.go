package vault

import (
	"context"
	"sync"
)

type Set struct {
	embedder Embedder
	cacheDir string

	mu       sync.Mutex
	warm     context.Context
	services map[string]*Service
	stops    map[string]context.CancelFunc
}

func NewSet(embedder Embedder, cacheDir string) *Set {
	return &Set{embedder: embedder, cacheDir: cacheDir, services: map[string]*Service{}, stops: map[string]context.CancelFunc{}}
}

// Configure keeps the index of every root whose path is unchanged, and cancels
// the build of every root it drops.
func (s *Set) Configure(profiles []Profile) error {
	if err := Validate(profiles); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := map[string]*Service{}
	for _, p := range profiles {
		if old := s.services[p.ID]; old != nil && old.profile == p {
			next[p.ID] = old
			continue
		}
		service, err := New(p, s.embedder, s.cacheDir)
		if err != nil {
			return err
		}
		next[p.ID] = service
	}
	for id, stop := range s.stops {
		if next[id] != s.services[id] {
			stop()
			delete(s.stops, id)
		}
	}
	s.services = next
	if s.warm != nil {
		s.warmLocked()
	}
	return nil
}

func (s *Set) warmLocked() {
	for id, service := range s.services {
		if s.stops[id] == nil {
			ctx, stop := context.WithCancel(s.warm)
			s.stops[id] = stop
			go service.Warm(ctx)
		}
	}
}

func (s *Set) Warm(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warm = ctx
	s.warmLocked()
}

// ForPath answers the root containing an already resolved path, or nil.
func (s *Set) ForPath(real string) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, service := range s.services {
		if contains(resolve(service.profile.Root), real) {
			return service
		}
	}
	return nil
}
