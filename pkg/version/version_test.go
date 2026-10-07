package version

import (
	"strings"
	"testing"
)

func TestVersionGet(t *testing.T) {
	info := Get()

	if info.Version != Version {
		t.Errorf("info.Version = %q, want %q", info.Version, Version)
	}
	if info.GitCommit != GitCommit {
		t.Errorf("info.GitCommit = %q, want %q", info.GitCommit, GitCommit)
	}
	if info.BuildDate != BuildDate {
		t.Errorf("info.BuildDate = %q, want %q", info.BuildDate, BuildDate)
	}
	if info.GoVersion == "" {
		t.Errorf("info.GoVersion is empty")
	}
	if info.Platform == "" {
		t.Errorf("info.Platform is empty")
	}
}

func TestVersionString(t *testing.T) {
	s := String()
	if !strings.HasPrefix(s, "v"+Version) {
		t.Errorf("String() = %q, expected prefix %q", s, "v"+Version)
	}
}
