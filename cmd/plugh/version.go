package main

import (
	"fmt"
	"runtime/debug"

	"golang.org/x/mod/semver"

	"github.com/stevenosloan/plugh/internal/voice"
)

// devel is the version of a build with no version stamped on it.
const devel = "(devel)"

// version is the module version this binary was built at: a tag like v0.1.0
// from go install or go run, a pseudo-version from a checkout, or devel. A
// var so tests can set it.
var version = buildVersion()

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return devel
}

// pinWarning says why plugh at version have might not handle a voice.yaml
// written for pin, or returns "" if it should. A pin is the oldest version a
// project needs: any later release on the same compatible line will do. That
// line is the major version, or the minor below v1, where minor releases can
// break things.
func pinWarning(pin, have string) string {
	if pin == "" || !semver.IsValid(have) {
		return "" // no pin, or a build that can't say what it is
	}
	if line(have) != line(pin) {
		return fmt.Sprintf("%s is written for plugh %s, and this is %s, which may read it differently; install a %s.x release",
			voice.RouterFile, pin, have, line(pin))
	}
	if semver.Compare(have, pin) < 0 {
		return fmt.Sprintf("%s needs plugh %s or later, and this is %s; upgrade it", voice.RouterFile, pin, have)
	}
	return ""
}

// line is the part of v that a compatible release keeps: v1 for v1.4.2, and
// v0.4 for v0.4.2.
func line(v string) string {
	if semver.Major(v) == "v0" {
		return semver.MajorMinor(v)
	}
	return semver.Major(v)
}
