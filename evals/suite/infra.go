package suite

import (
	"go.alis.build/evals/loadinfra"
)

// WithCloudRunTargets appends Cloud Run infra targets to a load suite.
func WithCloudRunTargets(targets ...loadinfra.CloudRunTarget) LoadSuiteOption {
	return func(s *LoadSuite) error {
		if len(targets) == 0 {
			return ErrInfraTargetsEmpty{Kind: "cloud run"}
		}
		entry := 0
		for _, t := range targets {
			if t.ID == "" || t.ProjectID == "" || t.Region == "" || t.ServiceName == "" {
				return ErrInfraCloudRunTargetIncomplete{ID: t.ID}
			}
			if t.Role == loadinfra.RoleEntry {
				entry++
			}
			s.cloudRun = append(s.cloudRun, t)
		}
		if entry != 1 {
			return ErrInfraCloudRunEntry{EntryCount: entry}
		}
		return nil
	}
}

// WithSpannerTargets appends Spanner infra targets to a load suite.
func WithSpannerTargets(targets ...loadinfra.SpannerTarget) LoadSuiteOption {
	return func(s *LoadSuite) error {
		if len(targets) == 0 {
			return ErrInfraTargetsEmpty{Kind: "spanner"}
		}
		for _, t := range targets {
			if t.ID == "" || t.ProjectID == "" || t.InstanceID == "" || t.Location == "" {
				return ErrInfraSpannerTargetIncomplete{ID: t.ID}
			}
			if t.Database == "" {
				return ErrInfraSpannerDatabase{ID: t.ID}
			}
			s.spanner = append(s.spanner, t)
		}
		return nil
	}
}

// ValidateInfraTargets checks cross-kind ID uniqueness after all suite options
// have been applied.
func ValidateInfraTargets(s *LoadSuite) error {
	return validateInfraTargetIDs(s.cloudRun, s.spanner)
}

// validateInfraTargetIDs rejects duplicate target IDs across Cloud Run and Spanner kinds.
func validateInfraTargetIDs(cloud []loadinfra.CloudRunTarget, spanner []loadinfra.SpannerTarget) error {
	seen := make(map[string]struct{}, len(cloud)+len(spanner))
	for _, t := range cloud {
		if _, ok := seen[t.ID]; ok {
			return ErrInfraDuplicateID{ID: t.ID}
		}
		seen[t.ID] = struct{}{}
	}
	for _, t := range spanner {
		if _, ok := seen[t.ID]; ok {
			return ErrInfraDuplicateID{ID: t.ID}
		}
		seen[t.ID] = struct{}{}
	}
	return nil
}

// CloudRunTargets returns a copy of declared Cloud Run infra targets.
func (s *LoadSuite) CloudRunTargets() []loadinfra.CloudRunTarget {
	if s == nil || len(s.cloudRun) == 0 {
		return nil
	}
	return append([]loadinfra.CloudRunTarget(nil), s.cloudRun...)
}

// SpannerTargets returns a copy of declared Spanner infra targets.
func (s *LoadSuite) SpannerTargets() []loadinfra.SpannerTarget {
	if s == nil || len(s.spanner) == 0 {
		return nil
	}
	return append([]loadinfra.SpannerTarget(nil), s.spanner...)
}

// HasInfraTargets reports whether the suite declares any infra observation targets.
func (s *LoadSuite) HasInfraTargets() bool {
	return s != nil && (len(s.cloudRun) > 0 || len(s.spanner) > 0)
}
