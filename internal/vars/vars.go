// Package vars is an internal technical variable store used at build time,
// populated with values ​​based on the state of the git repository.
package vars

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// License is the project license identifier.
const License = "AGPL-3.0"

var (
	// Name is the project name.
	Name = "MetricZ Exporter"

	// Version of application (git tag) semver/tag, e.g. v1.2.3
	Version = "dev"

	// Commit is the full git commit SHA.
	Commit = "unknown"

	// Revision build, count of commits
	Revision = 0

	// BuildTime is the UTC build start time.
	BuildTime = time.Unix(0, 0)

	// URL to repository (https)
	URL = "https://github.com/woozymasta/metricz-exporter"

	_revision  string
	_buildTime string
)

// BuildInfo contains build metadata.
type BuildInfo struct {
	// betteralign:ignore

	// Name is the project name.
	Name string `json:"name" example:"MetricZ Exporter"`

	// Version of application (git tag) semver/tag, e.g. v1.2.3
	Version string `json:"version" example:"v1.2.3"`

	// Commit is the full git commit SHA.
	Commit string `json:"commit" example:"da15c174cd2ada1ad247906536c101e8f6799def"`

	// Current git commit short SHA
	CommitShort string `json:"commit_short,omitempty" example:"da15c17"`

	// Revision build, count of commits
	Revision int `json:"revision,omitempty" example:"1337"`

	// BuildTime is the UTC build start time.
	BuildTime time.Time `json:"build_time,omitempty" example:"1970-01-01T00:00:00Z"`

	// URL to repository (https)
	URL string `json:"url,omitempty" example:"https://github.com/woozymasta/metricz-exporter"`

	// License
	License string `json:"license,omitempty" example:"AGPL-3.0"`
} //@name response.BuildInfo

func init() {
	if n, err := strconv.Atoi(_revision); err == nil {
		Revision = n
	}

	if _buildTime != "" {
		if t, err := time.Parse(time.RFC3339, _buildTime); err == nil {
			BuildTime = t.UTC()
		}
	}
}

// Print prints build info to stdout.
func Print() {
	fmt.Printf(`name:     %s
url:      %s
file:     %s
version:  %s
commit:   %s
revision: %d
built:    %s
license:  %s
`, Name, URL, os.Args[0], Version, Commit, Revision, BuildTime, License)
}

// Info returns full build info.
func Info() BuildInfo {
	return BuildInfo{
		Name:        Name,
		Version:     Version,
		Commit:      Commit,
		CommitShort: CommitShort(),
		Revision:    Revision,
		BuildTime:   BuildTime,
		URL:         URL,
		License:     License,
	}
}

// Ver returns minimal build info.
func Ver() BuildInfo {
	return BuildInfo{
		Name:     Name,
		Version:  Version,
		Commit:   Commit,
		Revision: Revision,
	}
}

// CommitShort returns short commit SHA.
func CommitShort() string {
	if len(Commit) > 7 {
		return Commit[:7]
	}

	return Commit
}
