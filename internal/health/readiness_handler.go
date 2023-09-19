package health

import "net/http"

// NewReadinessHandler returns a handler that, if you can reach it says that everything is hunky-dory.
func NewReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
	})
}
