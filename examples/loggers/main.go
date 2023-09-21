package main

import (
	"context"
	"runtime"
	"time"

	"andy.dev/srv"
)

func main() {
	srv.Declare(srv.ServiceInfo{
		Name: "loggers example",
	})
	srv.AddJob(logStuff)
	srv.Serve()
}

func logStuff(_ context.Context, log *srv.Logger) error {
	if runtime.GOOS == "darwin" {
		log.Info("waiting 4s, in case you need to hit approve", "GOOS", runtime.GOOS)
		time.Sleep(4 * time.Second)
	}
	for i := 0; i < 20; i++ {
		log.Debug("debug message")
		log.Info("info message")
		log.Warn("warn message")
		log.Error("error message")
		doWork()
	}
	return nil
}

func doWork() {
	time.Sleep(1 * time.Second)
}
