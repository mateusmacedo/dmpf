package observability

import "time"

// ShutdownGrace is how long a process waits for work in flight before it
// forces the exit. It lives in the root package because both the transport
// drain and the telemetry shutdown answer to the same window, and the root
// imports nothing — otelboot imports it, so it could not import back.
const ShutdownGrace = 10 * time.Second
