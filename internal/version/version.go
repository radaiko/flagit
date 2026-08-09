// Package version reports which build of Flagit is running.
//
// The value comes from one of two places, in this order:
//
//   - a runtime override, which is how a deployment supplies it: Coolify
//     exposes the deployed revision as SOURCE_COMMIT, and docker-compose.yml
//     maps that onto FLAGIT_COMMIT for the container.
//   - a build-time default, stamped in by the linker with
//     -ldflags "-X flagit/internal/version.Commit=<sha>", which is what
//     `make build` and a plain `docker build --build-arg GIT_COMMIT=…` do.
//
// Neither is guaranteed — a bare `go build` has no commit to offer — so the
// last resort is the explicit sentinel Unknown rather than an empty string
// that would render as a blank space in the dashboard.
package version

import "strings"

// Unknown is reported when no commit was supplied by either route.
const Unknown = "unknown"

// shortLength matches the abbreviation git itself prints.
const shortLength = 7

// Commit is the build-time revision, set by the linker. Left empty by an
// ordinary `go build`.
var Commit string

// Resolve settles on the commit to report. override wins so a redeploy of an
// unchanged image still names the revision it was deployed from.
//
// It is idempotent: resolving an already-resolved value returns it unchanged,
// so main and the HTTP handler can both call it.
func Resolve(override string) string {
	if v := strings.TrimSpace(override); v != "" {
		return v
	}
	if v := strings.TrimSpace(Commit); v != "" {
		return v
	}
	return Unknown
}

// Short abbreviates a full SHA the way git does. Anything that is not a long
// hex string — Unknown, a tag, a branch name — is returned whole, because
// truncating it would only make it harder to recognise.
func Short(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return Unknown
	}
	if len(commit) <= shortLength || !isHex(commit) {
		return commit
	}
	return commit[:shortLength]
}

// Known reports whether a real commit was supplied.
func Known(commit string) bool {
	commit = strings.TrimSpace(commit)
	return commit != "" && commit != Unknown
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
