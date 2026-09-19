// Package distkit is the distributed harness of KIT-06 for a producing
// context: it re-executes the test binary as two competing relays over a real
// broker and decides on what reached the topic. App block; tests build with
// the integration and distributed tags.
//
// The vector differs from the one of a consuming context. There is no inbox
// here and no deliberate redelivery, so DMPF-R004 has nothing to decide on;
// what two relays over one outbox can get wrong is draining the same record
// twice, and what the publication can get wrong is an envelope whose
// payload_hash no longer matches the payload it carries.
package distkit
