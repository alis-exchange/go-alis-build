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

// The registry is a process-global map guarded by mu. Registration
// typically happens once at package init, but tests and dynamic
// launchers may register from goroutines, so all accesses go through mu.
var (
	mu       sync.RWMutex
	registry = map[string]*Environment{}
)

// Register adds a named environment. Returns an error on duplicate names
// so callers can surface accidental double-registration at startup. Use
// [MustRegister] when a duplicate registration should halt the process.
func Register(name string, opts ...Option) error {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		return ErrDuplicateRegistration{Name: name}
	}
	e := &Environment{name: name}
	for _, opt := range opts {
		opt(e)
	}
	registry[name] = e
	return nil
}

// MustRegister is like [Register] but panics on error.
func MustRegister(name string, opts ...Option) {
	if err := Register(name, opts...); err != nil {
		panic(err)
	}
}

// WithSetup registers optional setup for the environment.
func WithSetup(h Hook) Option {
	return func(e *Environment) { e.setup = h }
}

// WithTeardown registers optional teardown for the environment.
func WithTeardown(h Hook) Option {
	return func(e *Environment) { e.teardown = h }
}

// Get returns a registered environment or nil when name is unknown.
func Get(name string) *Environment {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
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
