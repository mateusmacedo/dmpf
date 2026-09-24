// comment-discipline-ok-file: godoc de package; o que a admissão é e não é vem de FND-08 (RES-16, RES-17, MET-07), por exigência da spec do KRN-10.

// Package admission decides, before any work is done, whether a request enters
// the service: a token bucket for the rate and a ceiling for the concurrency,
// both declared per route and resolved per tenant (FND-08 RES-16).
//
// The refusal is categorized and cheap (RES-17): it happens before decode,
// validation or storage, and it names its reason so the provider can count it
// under MET-12. The tenant is never the raw value: it is resolved against the
// declared allowlist of observability/metrics, so every undeclared tenant
// shares one bucket under "other" and the key space stays bounded (MET-07).
//
// What this package does not contain: the outbound rate limit per dependency
// (RES-15, a decorator of KRN-09) and the limits of shape — message size,
// attribute count — which stay with FND-06.
package admission
