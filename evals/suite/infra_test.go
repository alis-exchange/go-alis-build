package suite

import (
	"testing"

	"go.alis.build/evals/loadinfra"
)

func TestWithCloudRunTargets_requiresEntry(t *testing.T) {
	t.Parallel()
	_, err := NewLoadSuite("s", WithCloudRunTargets(loadinfra.CloudRunTarget{
		ID: "dep", Role: loadinfra.RoleDependency,
		ProjectID: "p", Region: "r", ServiceName: "svc",
	}))
	if err == nil {
		t.Fatal("expected error without ENTRY target")
	}
}

func TestWithCloudRunTargets_andSpanner_duplicateID(t *testing.T) {
	t.Parallel()
	_, err := NewLoadSuite("s",
		WithCloudRunTargets(loadinfra.CloudRunTarget{
			ID: "same", Role: loadinfra.RoleEntry,
			ProjectID: "p", Region: "r", ServiceName: "svc",
		}),
		WithSpannerTargets(loadinfra.SpannerTarget{
			ID: "same", ProjectID: "p", InstanceID: "i", Location: "r", Database: "db",
		}),
	)
	if err == nil {
		t.Fatal("expected duplicate ID error")
	}
}

func TestWithSpannerTargets_requiresDatabase(t *testing.T) {
	t.Parallel()
	_, err := NewLoadSuite("s", WithSpannerTargets(loadinfra.SpannerTarget{
		ID: "db", ProjectID: "p", InstanceID: "i", Location: "r",
	}))
	if err == nil {
		t.Fatal("expected empty database error")
	}
}

func TestLoadSuite_infraTargetsCopied(t *testing.T) {
	t.Parallel()
	s := mustLoadSuite(t, "s",
		WithCloudRunTargets(loadinfra.CloudRunTarget{
			ID: "entry", Role: loadinfra.RoleEntry,
			ProjectID: "p", Region: "r", ServiceName: "svc",
		}),
		WithSpannerTargets(loadinfra.SpannerTarget{
			ID: "orders", ProjectID: "p", InstanceID: "i", Location: "r", Database: "orders",
		}),
	)
	if len(s.CloudRunTargets()) != 1 || len(s.SpannerTargets()) != 1 {
		t.Fatalf("cloud=%d spanner=%d", len(s.CloudRunTargets()), len(s.SpannerTargets()))
	}
	if !s.HasInfraTargets() {
		t.Fatal("HasInfraTargets=false, want true")
	}
}
