package srv

import "andy.dev/srv/log"

type funcEntry struct {
	f        func() error
	location log.CodeLocation
}

func (f *funcEntry) run() error {
	return f.f()
}
