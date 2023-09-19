/*
srv is a module for microservices providing:

  - structured logging (json/text/human-readable slog handlers)
  - profiling
  - metrics (prometheus)
  - health checks
  - lifecycle monitoring and management
  - live adjustment of logging levels
  - configuration via command-line flags
  - support for versioning via build tags

It is intended to cut down on the boilerplate
required for observability and good behavior in a container orchestration
environment.

It automatically provides the /metrics /readyz and /livez endpoints as well and
will instrument standard process stats and Go runtime stats by default.
If no readiness or health checks are configured, it will immedately report
itself as ready and will always return positive health and readiness checks.
*/

package srv
