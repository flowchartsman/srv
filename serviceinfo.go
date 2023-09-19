package srv

import (
	"errors"
	"log/slog"
	"regexp"
)

// ServiceInfo is a description for your service.
// Name is the only required field, and must be of the form
//
//	^[a-zA-Z][a-zA-Z0-9._-]*$
//
// System is optional, but can be used to group related services together in an aggregator. It must be of the same form as Name.
// Version, if provided, is required to be a valid semantic version following
// the format detailed in the [Semantic Versioning 2.0.0 Specification]. If you
// would like to pin the version number at buildtime, please see the docs for
// the [buildinfo] package.
// About is an optional field that will be printed if the service is called with --help.
//
// [Semantic Versioning 2.0.0 Specification]: https://semver.org/#semantic-versioning-200
type ServiceInfo struct {
	// Name is the name of the service. (REQUIRED)
	Name string
	// System is a name to collate the logs of the service with other, related services. (OPTIONAL)
	System string
	// Version is a valid Semantic Versioning version string. (OPTIONAL)
	Version string
	// About is any extra string data you'd like printed in the output of --help
	About string
}

// LogValue allows the service description to be logged canonically in [slog]
// output.
func (i ServiceInfo) LogValue() slog.Value {
	attrs := make([]slog.Attr, 1, 3)
	attrs[0] = slog.String("name", i.Name)
	if i.System != "" {
		attrs = append(attrs, slog.String("system", i.System))
	}
	if i.Version != "" {
		attrs = append(attrs, slog.String("version", i.Version))
	}
	return slog.GroupValue(attrs...)
}

// Ref: https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
// TODO: Strictly use for validation for now. Extract full info for UI/--version=full, when supported
var semverRx = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

var validName = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9._-]*$`)

func validateInfo(s ServiceInfo) error {
	if s.Name == "" {
		return errors.New("Name cannot be empty")
	}
	if !validName.MatchString(s.Name) {
		return errors.New("Name is invalid")
	}
	if s.System != "" && !validName.MatchString(s.System) {
		return errors.New("System name is invalid")
	}
	if s.Version != "" && !semverRx.MatchString(s.Version) {
		return errors.New("Version is invalid semver number")
	}
	return nil
}
