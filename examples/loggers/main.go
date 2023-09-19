package main

import (
	"context"
	"runtime"
	"time"

	"andy.dev/srv"
)

func main() {
	s, _ := srv.New(srv.ServiceInfo{
		Name: "loggers example",
	})
	s.Start(logStuff)
}

func logStuff(_ context.Context, log *srv.Logger) error {
	if runtime.GOOS == "darwin" {
		log.Info("waiting for four seconds in case you need to approve")
		time.Sleep(4 * time.Second)
	}
	for i := 0; i < 6; i++ {
		log.Debug("debug message")
		log.Info("info message")
		log.Warn("warn message")
		log.Error("error message")
		doWork()
	}
	return nil
}

func doWork() {
	time.Sleep(2 * time.Second)
}
