package main

import (
	"context"

	"andy.dev/srv"
)

func main() {
	srv.Declare(srv.ServiceInfo{
		Name:  "buildflags example",
		About: "This service has its version number assigned via build tags.",
		// Version: "0.0.1", // If you uncomment me, and use the makefile, srv will not like it.
	})
	srv.AddJob(logMessage)
	srv.Serve()
}

func logMessage(_ context.Context, log *srv.Logger) error {
	log.Info("you should see the version specified in the Makefile here...")
	return nil
}
