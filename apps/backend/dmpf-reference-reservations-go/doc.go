// comment-discipline-ok-file: godoc de package; o que a composition root é vem da matriz de blocos (ADR-015) e da topologia da SPEC-ACYKBF9V.

// Package dmpfreferencereservations is the composition root of the reservations
// context of the DMPF reference: one binary whose --role flag selects the gRPC
// api, the relay or the consumer, each wiring the concrete providers the
// ADR-015 permits only here.
package dmpfreferencereservations
