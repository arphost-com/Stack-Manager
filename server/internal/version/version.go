// Package version exposes the build's version so the server can report it
// (footer, /system/info) and agents can report theirs on check-in.
package version

// Base is the human release version. Keep it in sync with web/package.json's
// "version" field — they name the same release.
const Base = "1.5.7"

// GitSHA is stamped at build time via:
//
//	-ldflags "-X github.com/arphost-com/Stack-Manager/server/internal/version.GitSHA=<sha>"
//
// which the Dockerfile wires from the APP_GIT_SHA build arg (the same short
// commit the web build bakes as VITE_GIT_SHA). It is empty for a plain
// `go build`, so tests and ad-hoc builds report just the base version.
var GitSHA = ""

// Full returns the deployed version string, e.g. "1.5.0+ab12cd3", or just the
// base version when no SHA was stamped in.
func Full() string {
	if GitSHA == "" {
		return Base
	}
	return Base + "+" + GitSHA
}
