// Package env holds shared, named setup/teardown hooks that suites can
// declare a dependency on.
//
// An environment models expensive shared state — a seeded database, a
// warm cache, a spun-up in-process agent — that several suites might
// need. Registering it once and referencing it by name in each suite
// keeps activation cost paid once per LRO instead of once per suite.
//
// # Registration
//
// Register environments during package init:
//
//	func init() {
//	    env.Register("files-v2",
//	        env.WithSetup(initialiseFiles),
//	        env.WithTeardown(cleanupFiles),
//	    )
//	}
//
// Duplicate names panic — environments are process-global.
//
// # Activation
//
// The runner collects the union of `Environments()` from every suite
// selected for the run, activates them once (calling each `Setup` in
// registration order), executes suites, then tears down in reverse
// order. Setup failure surfaces as a setup-error result on every case in
// every suite that depends on that environment (or downstream of the
// failed environment).
//
// # Hooks
//
// [Hook] is a plain `func(context.Context) error`. Long-running setup
// should respect the context — the LRO cancels it on timeout/abort.
package env
