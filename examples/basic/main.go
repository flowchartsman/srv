package main

import (
	"context"
	"time"

	"andy.dev/srv"
	"andy.dev/srv/errors"
)

func main() {
	srv.Declare(srv.ServiceInfo{
		Name:   "basicsvc",
		System: "srv examples",
	})
	srv.AddShutdownHandler(shutdown)
	srv.AddJob(jobWillFail)
	srv.Serve()
}

func jobWillFail(_ context.Context, log *srv.Logger) error {
	for i := 1; i <= 5; i++ {
		if i < 5 {
			log.Info("I am going to die on message #5", "message_number", i)
		} else {
			log.Warn("Here I go!", "message_number", i)
		}
		doWork()
	}
	return errors.New("I warned you!")
}

func shutdown(_ context.Context, log *srv.Logger) error {
	log.Info("shutting down")
	doWork()
	return nil
}

func doWork() {
	time.Sleep(time.Second)
}
