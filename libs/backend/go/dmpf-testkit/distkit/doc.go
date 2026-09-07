// Package distkit is the distributed harness of KIT-06: it re-executes the test
// binary as producer and consumer over a real broker and injects a deliberate
// redelivery (V32), deciding DMPF-R004 on the final effect. App block; its
// tests carry the integration and distributed build tags.
package distkit
