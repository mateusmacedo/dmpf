// comment-discipline-ok-file: godoc de package; explica por que a composition root do consumo vive no bloco app (RFC §7.3), por exigência da spec do KRN-07.

// Package reservationsconsumer is the composition root of the reservations
// consumer: it binds the Postgres realizations to the application service of
// consumption and wraps it in the adapter of dmpfapp.
//
// It exists in the app block because it needs every other block at once — the
// contract to unpack OrderPlaced, the application for Consume, the provider for
// the transactional ports (RFC §7.3). Before KRN-07 this composition existed only
// inside a _test.go of the provider, outside the verifier's universe.
package reservationsconsumer
