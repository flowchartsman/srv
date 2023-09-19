package main

import (
	"context"

	"andy.dev/srv"
)

func main() {
	s, _ := srv.New(srv.ServiceInfo{
		Name:  "buildflags example",
		About: "This service has its version number assigned via build tags.",
	})
	s.Start(logMessage)
}

func logMessage(_ context.Context, log *srv.Logger) error {
	log.Info("you should see the version specified in the Makefile here...")
	return nil
}
