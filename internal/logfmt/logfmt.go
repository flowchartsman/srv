package logfmt

import (
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
)

// TODO: relocate to log package, mv fmtlocation to CodeLocation.String()

func FmtRecord(r slog.Record, trim bool) string {
	return FmtLocation(r.PC, trim)
}

func FmtLocation(pc uintptr, trim bool) string {
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.Line > 0 {
		if trim {
			f.File = filepath.Base(f.File)
		}
		return f.File + `:` + strconv.Itoa(f.Line)
	}
	return ""
}
