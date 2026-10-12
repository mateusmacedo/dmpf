package domain

// Instant is a point in time as nanoseconds since the Unix epoch: the outbox
// drains ordered by occurred_at (FND-04 §4.2), so seconds would collide under
// load. It arrives as a value resolved outside the domain (RFC §9.3).
type Instant int64
