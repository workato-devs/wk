package config

import "time"

// TimeoutDuration converts an integer seconds value to time.Duration.
func TimeoutDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// RegionURLs maps region identifiers to Workato base URLs.
var RegionURLs = map[string]string{
	"us": "https://www.workato.com",
	"eu": "https://app.eu.workato.com",
	"jp": "https://app.jp.workato.com",
	"au": "https://app.au.workato.com",
	"sg":    "https://app.sg.workato.com",
	"trial": "https://app.trial.workato.com",
}

// BaseURL returns the Workato base URL for the given region.
// Defaults to the US region if the region is unknown.
func BaseURL(region string) string {
	if url, ok := RegionURLs[region]; ok {
		return url
	}
	return RegionURLs["us"]
}

const (
	DefaultRegion  = "us"
	DefaultTimeout = 30 // seconds
	APIPathPrefix  = "/api"
)
