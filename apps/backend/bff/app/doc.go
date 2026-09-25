// comment-discipline-ok-file: godoc de package; o papel do BFF vem de ADR-024 (REST externo, gRPC interno) e de ADR-015 (provider concreto só na composition root).

// Package app is the public edge of the DMPF reference topology:
// the only REST/JSON surface (ADR-024), translating each route into a gRPC call
// to the orders or reservations context, with no domain of its own.
package app
