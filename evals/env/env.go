package env

import (
	"context"
	"sync"
)

// Hook runs once per environment activation in a test/eval batch.
type Hook func(context.Context) error

// Environment is a named shared dependency (for example a seeded database).
type Environment struct {
	name     string
	setup    Hook
	teardown Hook
}

// Option configures an environment at registration time.
type Option func(*Environment)

// Registry holds named environments. [DefaultRegistry] is the process-wide
// registry; [New] returns an isolated registry for tests.
type Registry struct {
	mu   sync.RWMutex
	envs map[string]*Environment
}

var defaultRegistry = New()

// New returns an empty environment registry.
func New() *Registry {
	return &Registry{
		envs: make(map[string]*Environment),
	}
}

// DefaultRegistry returns the process-wide environment registry used by
// package-level Register and Get.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// Register adds a named environment to DefaultRegistry. Returns an error on
// duplicate names so callers can surface accidental double-registration at
// startup. Use [MustRegister] when a duplicate registration should halt the
// process.
func Register(name string, opts ...Option) error {
	return defaultRegistry.Register(name, opts...)
}

// MustRegister is like [Register] but panics on error.
func MustRegister(name string, opts ...Option) {
	if err := Register(name, opts...); err != nil {
		panic(err)
	}
}

// Get returns a registered environment from DefaultRegistry or nil when
// name is unknown.
func Get(name string) *Environment {
	return defaultRegistry.Get(name)
}

// Register adds a named environment. Returns [ErrDuplicateRegistration]
// when name is already registered in this registry.
func (r *Registry) Register(name string, opts ...Option) error {
	if r == nil {
		return ErrNotConfigured{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.envs[name]; exists {
		return ErrDuplicateRegistration{Name: name}
	}
	e := &Environment{name: name}
	for _, opt := range opts {
		if opt == nil {
			return ErrNilOption{}
		}
		opt(e)
	}
	r.envs[name] = e
	return nil
}

// Get returns a registered environment or nil when name is unknown.
func (r *Registry) Get(name string) *Environment {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.envs[name]
}

// WithSetup registers optional setup for the environment.
func WithSetup(h Hook) Option {
	return func(e *Environment) { e.setup = h }
}

// WithTeardown registers optional teardown for the environment.
func WithTeardown(h Hook) Option {
	return func(e *Environment) { e.teardown = h }
}

// Name returns the registered environment name.
func (e *Environment) Name() string {
	if e != nil {
		return e.name
	}
	return ""
}

// Setup returns the environment's setup hook, or nil when none was set.
func (e *Environment) Setup() Hook {
	if e != nil {
		return e.setup
	}
	return nil
}

// Teardown returns the environment's teardown hook, or nil when none was set.
func (e *Environment) Teardown() Hook {
	if e != nil {
		return e.teardown
	}
	return nil
}
