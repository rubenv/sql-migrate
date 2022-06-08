package migrate

import (
	_ "embed"
)

//go:embed VERSION
var version string

// GetVersion returns version depending on the VERSION file
func GetVersion() string {
	return version
}
