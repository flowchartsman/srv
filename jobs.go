package srv

import (
	"context"

	"andy.dev/srv/log"
)

// TaskFn is the basic function type that drives jobs, health checks and so on.
type TaskFn func(context.Context, *Logger) error

// Job represents a task to be run and tracked by the [Srv] instance.
// On start, the Run method will be invoked with a context that will be
// cancelled in case of shutdown as well as a [*Logger]. A return value of nil
// marks the job as completed, while an error marks the job as failed.
type Job interface {
	Run(context.Context, *Logger) error
}

// Task creates a TaskFn function that takes an additional argument of any type.
//
// Example:
//
//		func myjob(ctx context.Context, log *srv.Logger input *MyType) error {
//		  // ...
//		}
//
//	 srv.AddJob(srv.JobFn(Task(myjobFn, &MyType{/*...*/})))
func Task[T any](fn func(context.Context, *Logger, T) error, arg T) TaskFn {
	return func(ctx context.Context, logger *Logger) error {
		return fn(ctx, logger, arg)
	}
}

type jobFn TaskFn

func (j jobFn) Run(ctx context.Context, log *Logger) error {
	return j(ctx, log)
}

func (s *Srv) AddJobs(jobs ...Job) {
	// TODO: can't fatal here if the type is locked. Better to return error and
	// move Srv to unexported instance
	// s.mu.Lock()
	// defer s.mu.Unlock()
	if s.started {
		s.fatal(log.Up(1), "AddJobs called after Start()")
	}
	s.jobs = append(s.jobs, jobs...)
}

// AddJobFns allows one or more [TaskFn] functions to be added as jobs.
func (s *Srv) AddJobFn(fn TaskFn) {
	s.AddJobs(jobFn(fn))
}
