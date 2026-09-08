// comment-discipline-ok-file: godoc de package; o que cada kit prova e em que bloco vive vem de FND-09 §4–§10 e da matriz de blocos (RFC §7.4), por exigência da spec do KRN-11.

// Package dmpftestkit is the DMPF test instrument fixed by FND-09: one kit per
// layer of the pyramid (domainkit, serviceskit, providerkit, appkit, distkit),
// the golden-fixture loader with its three oracles reported apart (golden), the
// dependency rule as a fitness function over the production universe (fitness),
// the determinism primitives every kit shares (clock, ids, stable) and the
// testing.TB adapter that turns a Verdict into a test failure (tb).
//
// The kit is production code to the conformance checker (FIT-01: only _test.go
// files stay out of the universe), so each package declares the block its edges
// allow — domainkit is domain, golden is contract, serviceskit, providerkit,
// clock, ids and stable are provider, and appkit, distkit, fitness and tb are
// app. No kit depends on testing, os or time except through tb and the two
// app harnesses; that is what lets a domain test that needs an infrastructure
// double fail by naming the dependency (V29, V30).
package dmpftestkit
