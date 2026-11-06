// Package version carries build information set through ldflags by the Taskfile.
package version

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// String formats the build information for the command line.
func String() string {
	return Version + " (" + Commit + ", " + BuildDate + ")"
}
