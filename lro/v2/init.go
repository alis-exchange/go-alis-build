package lro

import (
	"fmt"
	"os"
	"strings"
)

var requiredEnvs = []string{
	"ALIS_MANAGED_SPANNER_PROJECT",
	"ALIS_MANAGED_SPANNER_INSTANCE",
	"ALIS_MANAGED_SPANNER_DB",
	"ALIS_OS_PROJECT",
	"ALIS_REGION",
}

// init validates the environment required by the lro package at import time.
func init() {
	missing := make([]string, 0, len(requiredEnvs))
	for _, key := range requiredEnvs {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("go.alis.build/lro/v2 requires env vars: %s", strings.Join(missing, ", ")))
	}
}
