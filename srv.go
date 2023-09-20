package srv

import "sync"

var (
	instanceMu sync.Mutex
	inst       *instance
)
