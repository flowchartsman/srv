package main

import (
	"context"
	"runtime"
	"time"

	"andy.dev/srv"
	"andy.dev/srv/errors"
)

func main() {
	srv.Declare(srv.ServiceInfo{
		Name: "healthcheck example",
	})
	srv.AddHealthCheck("Fail On 3", alwaysFail,
		srv.MaxFailures(3),
		srv.Interval(5*time.Second),
		srv.Timeout(2*time.Second))
	srv.AddJob(runForever)
	srv.Serve()
}

func alwaysFail(_ context.Context, log *srv.Logger) error {
	doWork()
	return errors.New("I don't feel so good")
}

func runForever(_ context.Context, log *srv.Logger) error {
	if runtime.GOOS == "darwin" {
		log.Info("waiting 4s, in case you need to hit approve", "GOOS", runtime.GOOS)
		time.Sleep(4 * time.Second)
	}
	for {
		log.Info("just me running forever")
		doWork()
	}
}

func doWork() {
	time.Sleep(1 * time.Second)
}
