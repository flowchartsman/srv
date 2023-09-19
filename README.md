# srv - a way to do microservices

## Health

### Startup
`srv` provides a `/startupz` route on `:8081`  that will become active the instant `srv.New` is called. It will respond with `503 Service Unavailable` until `srv.Start()` has completed instrumentation and starting any jobs. Thereafter it will respond with `200 OK`.

### Readiness

`srv` provides a very basic readiness handler at `/readyz` that will start responding after the call to `srv.Start()`. The route simply responds with `200` and `OK` as the response body to every request. It will never explicitly fail, with the reasoning being that if your service is so busy that it can't respond to such a simple request in whatever timeout you deem appropriate then it isn't "ready".

Readiness handlers can cause

## TODO
- Leadership
- https://github.com/felixge/fgprof