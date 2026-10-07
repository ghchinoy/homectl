package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current semantic version of homectl.
	Version = "0.3.0"
	// GitCommit is the git commit hash, populated via -ldflags.
	GitCommit = "dev"
	// BuildDate is the ISO8601 build timestamp, populated via -ldflags.
	BuildDate = "unknown"
)

// Info holds detailed build and runtime version information.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
}

// Get returns the current build and runtime version details.
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a single-line summary of version details.
func String() string {
	return fmt.Sprintf("v%s (commit: %s, built: %s, %s %s/%s)",
		Version, GitCommit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
