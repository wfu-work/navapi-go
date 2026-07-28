package services

import (
	"regexp"
	"strings"
)

var providerAPIVersionPrefix = regexp.MustCompile(`(?i)^/(v[0-9]+(?:beta[0-9]*)?)(?:/|$)`)

// joinProviderEndpoint accepts provider base URLs both with and without an API
// version suffix. This matches the formats accepted by the provider form.
func joinProviderEndpoint(baseURL string, upstreamPath string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	upstreamPath = "/" + strings.TrimLeft(strings.TrimSpace(upstreamPath), "/")
	if baseURL == "" {
		return upstreamPath
	}

	match := providerAPIVersionPrefix.FindStringSubmatch(upstreamPath)
	if len(match) > 1 && strings.HasSuffix(strings.ToLower(baseURL), "/"+strings.ToLower(match[1])) {
		upstreamPath = upstreamPath[len(match[1])+1:]
		if upstreamPath == "" {
			upstreamPath = "/"
		}
	}
	return baseURL + upstreamPath
}
