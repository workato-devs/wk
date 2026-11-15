package telemetry

import "os"

// IsDisabled returns true if telemetry should be suppressed. Checks all three
// opt-out surfaces defined in ADR-010 Decision 3, in order:
//
//  1. DO_NOT_TRACK=1 — Console Do Not Track standard (https://consoledonottrack.com/)
//  2. WK_TELEMETRY_DISABLED=1 — wk-specific env var (useful in CI)
//  3. ~/.wk/telemetry.json Disabled field — set by `wk telemetry disable`
func IsDisabled() bool {
	return disabledByEnv() || disabledByConfig()
}

// IsCI returns true when running inside a known CI environment. When true,
// the client sets the wk.ci=true span attribute (ADR-010 Decision 4).
// CI events are tagged, not suppressed — CI usage is real signal.
func IsCI() bool {
	ciVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"BUILDKITE",
		"CI_NAME",
		"TRAVIS",
		"JENKINS_URL",
		"TEAMCITY_VERSION",
		"DRONE",
		"BITBUCKET_BUILD_NUMBER",
		"TF_BUILD", // Azure DevOps
	}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}
