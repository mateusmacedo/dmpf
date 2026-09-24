// Package distkit is the distributed harness of KIT-06: it re-executes the test binary as
// producer and consumer over a real broker, injects a deliberate redelivery (V32) and decides
// DMPF-R004 on the final effect. App block; tests build with the integration and distributed tags.
package distkit
