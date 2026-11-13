package main

import (
	"fmt"
	"io"
	"runtime/debug"
)

// version is stamped at release time: -ldflags "-X main.version=1.2.3".
// "dev" means nobody stamped it (go build, go run).
var version = "dev"

var versionCmd = command{
	name:    "version",
	usage:   "version",
	summary: "print the version",
	run:     runVersion,
}

func runVersion(_ []string, out io.Writer) error {
	module := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		module = bi.Main.Version
	}
	fmt.Fprintf(out, "vantage %s\n", resolveVersion(version, module))
	return nil
}

// resolveVersion prefers the stamped release version. Without one it falls
// back to the module version that `go install ...@vX.Y.Z` records, and last
// to "dev".
func resolveVersion(stamped, module string) string {
	if stamped != "dev" {
		return stamped
	}
	if module != "" && module != "(devel)" {
		return module
	}
	return "dev"
}
