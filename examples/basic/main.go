package main

import (
	"context"
	"time"

	"andy.dev/srv"
	"andy.dev/srv/errors"
)

func main() {
	s, _ := srv.New(srv.ServiceInfo{
		Name:   "basicsvc",
		System: "srv examples",
	})

	// supports flags
	var errMsg string
	s.FlagStringVar(&errMsg, "err-msg", "I warned you!", "the error to return from the job")
	s.ParseFlags()

	s.AddJobFn(srv.Task(jobWillFail, errMsg))
	s.AddShutdownHandlers(shutdown)
	s.Start()
}

func jobWillFail(_ context.Context, log *srv.Logger, failMsg string) error {
	for i := 1; i <= 5; i++ {
		if i < 5 {
			log.Info("I am going to fail on message #5", "message_number", i)
		} else {
			log.Warn("Here I go!", "message_number", i)
		}
		doWork()
	}
	return errors.New(failMsg)
}

func shutdown(_ context.Context, log *srv.Logger) error {
	log.Info("shutting down")
	doWork()
	return nil
}

func doWork() {
	time.Sleep(time.Second)
}
