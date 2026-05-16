package daemon

import (
	"context"
	"sync"

	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/lazycommit/lazycommit/internal/state"
)

type Repository struct {
	Config       config.RepoConfig
	Cancel       context.CancelFunc
	StateMachine *state.StateMachine
}

type Registry struct {
	repos map[string]*Repository
	mu    sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		repos: make(map[string]*Repository),
	}
}

func (r *Registry) Add(repo config.RepoConfig, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repos[repo.Path] = &Repository{Config: repo, Cancel: cancel}
}

func (r *Registry) SetStateMachine(path string, sm *state.StateMachine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if repo, ok := r.repos[path]; ok {
		repo.StateMachine = sm
	}
}

func (r *Registry) Remove(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.repos, path)
}

func (r *Registry) Get(path string) (*Repository, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	repo, ok := r.repos[path]
	return repo, ok
}

func (r *Registry) All() []*Repository {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]*Repository, 0, len(r.repos))
	for _, repo := range r.repos {
		all = append(all, repo)
	}
	return all
}
