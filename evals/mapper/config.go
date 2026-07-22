package mapper

import "os"

// Config carries mapper-wide defaults set once at process bootstrap.
type Config struct {
	// GoogleProjectID is stamped on every mapped Run as google_project_id.
	// When empty, [googleProjectID] falls back to ALIS_OS_PROJECT for
	// backward compatibility until products call [SetConfig] at startup.
	GoogleProjectID string
}

var defaultConfig Config

// SetConfig replaces the package-level mapper configuration.
func SetConfig(c Config) {
	defaultConfig = c
}

// googleProjectID returns the configured project id, or ALIS_OS_PROJECT when
// unset. Explicit SetConfig values take precedence over the environment.
func googleProjectID() string {
	if defaultConfig.GoogleProjectID != "" {
		return defaultConfig.GoogleProjectID
	}
	return os.Getenv("ALIS_OS_PROJECT")
}
