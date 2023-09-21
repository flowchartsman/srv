package main

import (
	"context"
	"time"

	"andy.dev/srv"
	"andy.dev/srv/errors"
)

func main() {
	srv.Declare(srv.ServiceInfo{
		Name: "shutdown_example",
	})
	srv.AddJob(printStuff)
	srv.AddShutdownHandler(
		shutdownGood,
		srv.Job(shutdownBad, "something went wrong"),
		shutdownPanic,
		shutdownSkipped,
	)
	srv.Serve()
}

func printStuff(_ context.Context, log *srv.Logger) error {
	for i := 1; i <= 3; i++ {
		log.Infof("message #%d", i)
		doWork()
	}
	return nil
}

func shutdownGood(_ context.Context, log *srv.Logger) error {
	log.Info("shutting down well")
	doWork()
	return nil
}

func shutdownBad(_ context.Context, log *srv.Logger, errMsg string) error {
	log.Info("shutting down poorly")
	doWork()
	return errors.New(errMsg)
}

func shutdownPanic(_ context.Context, log *srv.Logger) error {
	log.Warn("I am fixing to panic")
	doWork()
	panic("OH NO!")
}

func shutdownSkipped(_ context.Context, log *srv.Logger) error {
	log.Info("You won't see me, but I would have succeeded")
	doWork()
	return nil
}

func doWork() {
	time.Sleep(time.Second)
}
