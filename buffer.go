package srv

import (
	"bytes"
	"sync"
)

// buffer pool to reduce GC
var buffers = sync.Pool{
	// New is called when a new instance is needed
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GetBuffer fetches a buffer from the pool
func GetBuf() *bytes.Buffer {
	return buffers.Get().(*bytes.Buffer)
}

// PutBuffer returns a buffer to the pool
// make sure to call this with defer()
func PutBuf(buf *bytes.Buffer) {
	buf.Reset()
	buffers.Put(buf)
}
