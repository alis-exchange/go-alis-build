package adk

import (
	"strings"
)

// AudienceFromBaseURL returns the Cloud Run ID token audience for a target
// base URL. It is a URL-parsing helper kept here for consumers that mint
// their own ID tokens against a Cloud Run-hosted ADK sublauncher; the ADK
// client itself does not use it.
func AudienceFromBaseURL(baseURL string) (string, error) {
	host, err := hostFromBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return "https://" + host, nil
}

func hostFromBaseURL(baseURL string) (string, error) {
	s := strings.TrimSpace(baseURL)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if s == "" {
		return "", ErrEmptyBaseURL{}
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "", ErrInvalidBaseURL{URL: baseURL}
	}
	return s, nil
}
