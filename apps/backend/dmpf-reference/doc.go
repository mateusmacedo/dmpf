// comment-discipline-ok-file: godoc de package; o que a composition root é e não é vem da matriz de blocos (RFC §7.3, ADR-015), por exigência da spec do KRN-12.

// Package dmpfreference is the reference composition root of the DMPF kernel:
// one binary whose --role flag selects the api, relay or consumer process,
// each wiring the concrete providers the ADR-015 permits only here.
package dmpfreference
