package srv

import "context"

// JobFn is a job to be run at startup. The service will run until either a
// single job has registered an error or until all jobs have completed
// successfully. A logger will be automatically supplied to your function for
// logging messages.
type JobFn func(context.Context, *Logger) error

// Job creates a JobFn from a function that takes a context and a [*Logger]
// along with an additional argument of any type.
//
// Example:
//
//	func myjobFn(ctx context.Context, log *srv.Logger input *MyType) error {
//	  // ...
//	}
//
//	srv.Start(srv.Job(myjobFn, &MyType{/*...*/}))
func Job[T any](fn func(context.Context, *Logger, T) error, arg T) JobFn {
	return func(ctx context.Context, logger *Logger) error {
		return fn(ctx, logger, arg)
	}
}
