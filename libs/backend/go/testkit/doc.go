// comment-discipline-ok-file: godoc de package; o que cada kit prova e em que bloco vive vem de FND-09 §4–§10 e da matriz de blocos (RFC §7.4), por exigência da spec do KRN-11.

// Package testkit is the DMPF test instrument fixed by FND-09: one kit per
// layer of the pyramid (domainkit, serviceskit, providerkit, and the appkit and
// distkit each context writes over tb/dist), the golden-fixture loader with its
// three oracles reported apart (golden), the dependency rule as a fitness
// function over the production universe (fitness), the determinism primitives
// every kit shares (clock, ids, stable), the testing.TB adapter that turns a
// Verdict into a test failure (tb) and its siblings for processes (tb/dist),
// telemetry (obstest), the execution context (tb/execctx) and the reference
// use case of the libraries (tb/reference).
//
// The kit is production code to the conformance checker (FIT-01: only _test.go
// files stay out of the universe), so each package declares the block its edges
// allow — domainkit is domain, golden is contract, serviceskit, providerkit,
// clock, ids and stable are provider, and fitness, tb and its siblings are
// app. No kit depends on testing, os or time except through tb and its
// siblings, which neither tb nor domainkit import; that is what lets a domain
// test that needs an infrastructure double fail by naming the dependency (V29,
// V30).
package testkit
