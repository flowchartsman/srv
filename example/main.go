package main

import (
	"context"
	"fmt"
	"time"

	"andy.dev/srv"
	"andy.dev/srv/errors"
)

func main() {
	srv.Info("oops I logged early!")
	srv.Init("mysystem/myservice",
		srv.WithLogLevel(srv.LevelDebug),
		srv.WithMetrics(true),
		srv.WithStartup(true),
		srv.DebugMetricsInterval(10*time.Second),
	)

	srv.AddShutdownHandler(func() error {
		srv.Info("doing handler stuff")
		time.Sleep(2 * time.Second)
		return nil
	})

	srv.AddShutdownHandler(func() error {
		srv.Info("doing handler stuff that panics")
		panic("oh noes")
	})

	srv.Info("connecting to something")
	time.Sleep(3 * time.Second)
	srv.Started()

	srv.Go(work)
	srv.Wait()
}

func work(context.Context) error {
	srv.AddShutdownHandler(func() error {
		srv.Info("you won't see me")
		time.Sleep(2 * time.Second)
		return nil
	})

	goodVals := srv.NewCounter("values_total", `total count of goodvalues`)

	workChan := make(chan int)
	srv.Go(func(ctx context.Context) error {
		i := 1
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-t.C:
				workChan <- i
				i++
			case <-ctx.Done():
				return nil
			}
		}
	})

	for {
		select {
		case i := <-workChan:
			srv.Infof("doing some work #%d", i)
			if i%20 == 0 {
				err := srv.UnavailableWhile("doing some long work", doSomethingImportant)
				if err != nil {
					return fmt.Errorf("something important failed: %w", err)
				}
			}
			if i%8 == 0 {
				srv.Error("value is bad", "value", i)
			} else {
				goodVals.Add(1)
			}
		case <-srv.Done():
			srv.Info("looks like it's time to go")
			return nil
		}
	}
}

var importantCt int

func doSomethingImportant() error {
	importantCt++
	srv.Info("doing something important")
	if importantCt == 2 {
		return errors.New("all good things must come to an end")
	}
	time.Sleep(10 * time.Second)
	srv.Info("finished important stuff")
	return nil
}
