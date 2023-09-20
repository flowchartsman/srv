package srv

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// TODO: full semver breakdown and module stuff for format %+s call.
type buildData struct {
	version string
	// libSrvVersion string
	goversion   string
	goos        string
	goarch      string
	vcs         string
	vcsTime     time.Time
	vcsRevision string
	vcsDirty    bool
}

func (s *instance) getBuildData() *buildData {
	bd := &buildData{
		version: "<no version>",
	}
	if s.srvInfo.Version != "" {
		bd.version = s.srvInfo.Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		bd.goversion = bi.GoVersion
		for _, s := range bi.Settings {
			switch s.Key {
			case "GOOS":
				bd.goos = s.Value
			case "GOARCH":
				bd.goarch = s.Value
			case "vcs":
				bd.vcs = s.Value
			case "vcs.time":
				vcsTime, err := time.Parse(time.RFC3339, s.Value)
				if err == nil {
					bd.vcsTime = vcsTime
				}
			case "vcs.revision":
				bd.vcsRevision = s.Value
			case "vcs.modified":
				bd.vcsDirty, _ = strconv.ParseBool(s.Value)
			}
		}
	}
	return bd
}

func (bd *buildData) String() string {
	var sb strings.Builder
	sb.WriteString(bd.version)
	// take goversion as a proxy for build data being available
	if bd.goversion != "" {
		sb.WriteString(fmt.Sprintf(" (%s %s/%s)", bd.goversion, bd.goos, bd.goarch))
		// and bd.vcs as a proxy for VCS data being available
		if bd.vcs != "" {
			sb.WriteString(fmt.Sprintf(" %s revision: %s", bd.vcs, bd.vcsRevision))
			if bd.vcsDirty {
				sb.WriteString(" (dirty)")
			}
			sb.WriteString(" " + bd.vcsTime.Format(time.DateTime))
		}
	}
	return sb.String()
}

// func (bd *buildData) Format(state fmt.State, verb rune) {
// 	switch verb {
// 	case 'q':
// 		fmt.Fprintf(state, "%q", bd.String())
// 	case 's':
// 		fmt.Fprint(state, bd.String())
// 		// switch {
// 		// case state.Flag('+'):
// 		// 	//TODO
// 		// }
// 	}
// }
