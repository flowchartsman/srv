/*
srv is a module for microservices providing logging, metrics and
lifecycle and health monitoring. It is intended to cut down on the boilerplate
required for observability and good behavior in a container orchestration
environment.

It automatically provides the /metrics /readyz and /livez endpoints, and
will instrument standard process stats and Go runtime stats by default.
If no readiness or health checks are configured, it will immedately report
itself as ready and will always return positive health and readiness checks.

All Prometheus metrics supported by the official Go client are
exposed via methods on the type, and will be automatically registered.

It also provides a global slog.Logger which is set up to
print JSON-formatted messages on os.Stdout.

It provides a context for passing to calls which accept it, and this will be
marked done if the process receives SIGINT.
*/

package srv
