package flows

import "regexp"

var (
	StepIdRegex   = regexp.MustCompile(`^[^-]+$`)
	ParentIdRegex = regexp.MustCompile(`^([a-z0-9]+)-([^-]+)$`)
)
