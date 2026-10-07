package spannertest

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	tcspanner "github.com/testcontainers/testcontainers-go/modules/gcloud/spanner"
)

// DefaultImage is the emulator image started when SPANNERTEST_EMULATOR is
// set; SPANNERTEST_EMULATOR_IMAGE overrides it. It is pinned rather than
// :latest so CI and laptops run the same Spanner build.
const DefaultImage = "gcr.io/cloud-spanner-emulator/emulator:1.5.57"

const (
	// hostEnv names a running emulator to reuse.
	hostEnv = "SPANNER_EMULATOR_HOST"
	// enableEnv, set to any value, starts an emulator in Docker.
	enableEnv = "SPANNERTEST_EMULATOR"
	// imageEnv overrides DefaultImage.
	imageEnv = "SPANNERTEST_EMULATOR_IMAGE"
)

var (
	// startOnce guards the one container start per test binary.
	startOnce sync.Once
	// startedHost is the address of the container startOnce started.
	startedHost string
	// startErr is why the container failed to start, if it did.
	startErr error
	// startCount counts container starts; tests assert it stays at most 1.
	startCount atomic.Int32
)

// Host returns the address of the emulator to test against:
// SPANNER_EMULATOR_HOST when it is set, else a container started once per
// test binary when SPANNERTEST_EMULATOR is set. With neither set it skips
// t. It fails t when the container cannot start, since the run opted in.
func Host(t testing.TB) string {
	t.Helper()
	host, start, skip := resolve(os.Getenv)
	switch {
	case host != "":
		return host
	case !start:
		t.Skip(skip)
	}
	startOnce.Do(func() {
		// Preset so a panic inside startContainer leaves a failure behind
		// for every later caller, not an empty host and a nil error.
		startErr = errors.New("starting the Spanner emulator did not complete")
		startedHost, startErr = startContainer(context.Background())
	})
	if startErr != nil {
		t.Fatalf("spannertest: %v (is Docker running?)", startErr)
	}
	return startedHost
}

// resolve applies the start rules to getenv and returns the host to reuse,
// whether to start a container, or the message to skip with.
func resolve(getenv func(string) string) (host string, start bool, skip string) {
	if h := getenv(hostEnv); h != "" {
		return h, false, ""
	}
	if getenv(enableEnv) != "" {
		return "", true, ""
	}
	return "", false, fmt.Sprintf(
		"spannertest: set %s to use a running Spanner emulator, or %s=1 to start one in Docker", hostEnv, enableEnv,
	)
}

// startContainer runs the emulator image and returns its address. The
// testcontainers reaper removes the container when the test binary exits.
func startContainer(ctx context.Context) (string, error) {
	startCount.Add(1)
	img := cmp.Or(os.Getenv(imageEnv), DefaultImage)
	ctr, err := tcspanner.Run(ctx, img)
	if err != nil {
		return "", fmt.Errorf("starting the Spanner emulator (%s): %w", img, err)
	}
	return ctr.URI(), nil
}
