/*
Package buildinfo exists to provide a way to set build information at
compile-time with ldflags like so:

	LDFLAGS=(
	  "-X 'andy.dev/srv/buildinfo.Version=${VERSION}'"
	) && \
	go build -ldflags="${LDFLAGS[*]}"

*/

package buildinfo

// Version is the build-time version for your service. If this value is set, you
// should not pass a version string when calling [srv.New], as srv will panic,
// rather than allow you to run with with inconsistent versioning.
var Version string
