// Package provider is the provider block of the orders bounded context: the
// PostgreSQL repository over the Order snapshot, with optimistic locking in
// SQL, and the mapper from the two domain events into the
// company.orders.event.v1 contracts. It never opens its own transaction: every
// query runs on the pgx.Tx a postgres.Tx exposes.
// Its unit and its bounded context are stated in dmpf-units.json (ADR-012).
package provider
