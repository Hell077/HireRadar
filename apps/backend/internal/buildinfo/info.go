// Package buildinfo exposes values embedded in a backend binary at build time.
package buildinfo

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)
