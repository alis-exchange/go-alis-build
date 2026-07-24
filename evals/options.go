package evals

import "go.alis.build/evals/report"

// RunOption configures one suite invocation.
type RunOption interface {
	apply(*runConfig) error
}

type runConfig struct {
	maxConcurrency int
	reporter       report.Reporter
	batchID        string
	operation      string
	googleProject  string
}

func defaultRunConfig() runConfig {
	return runConfig{maxConcurrency: 1}
}

func applyRunOptions(base runConfig, opts []RunOption) (runConfig, error) {
	cfg := base
	var agg ConfigErrors
	for _, opt := range opts {
		if opt == nil {
			agg.add(ErrNilOption{})
			continue
		}
		if err := opt.apply(&cfg); err != nil {
			agg.add(err)
		}
	}
	if len(agg.errs) > 0 {
		return cfg, &agg
	}
	return cfg, nil
}

// WithMaxConcurrency overrides the peak number of concurrently active cases.
func WithMaxConcurrency(n int) RunOption {
	return runOptionFunc(func(cfg *runConfig) error {
		if n <= 0 {
			return ErrInvalidConcurrency{Value: n}
		}
		cfg.maxConcurrency = n
		return nil
	})
}

// WithReporter replaces the default reporter used by RunAndPublish.
func WithReporter(r report.Reporter) RunOption {
	return runOptionFunc(func(cfg *runConfig) error {
		if r == nil {
			return ErrNilReporter{}
		}
		cfg.reporter = r
		return nil
	})
}

// WithBatchID stamps Run.batch_id when non-empty.
func WithBatchID(id string) RunOption {
	return runOptionFunc(func(cfg *runConfig) error {
		cfg.batchID = id
		return nil
	})
}

// WithOperation stamps Run.operation when non-empty.
func WithOperation(name string) RunOption {
	return runOptionFunc(func(cfg *runConfig) error {
		cfg.operation = name
		return nil
	})
}

// WithGoogleProjectID overrides ALIS_OS_PROJECT for Run.google_project_id.
func WithGoogleProjectID(id string) RunOption {
	return runOptionFunc(func(cfg *runConfig) error {
		cfg.googleProject = id
		return nil
	})
}

type runOptionFunc func(*runConfig) error

func (f runOptionFunc) apply(cfg *runConfig) error { return f(cfg) }
