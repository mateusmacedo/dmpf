// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), e a garantia WORM vem da SPEC-CWBA4BPV.

// Package s3 realizes ports.ObjectStore over Amazon S3 with Object Lock: every
// write carries its SHA-256 checksum, the declared encryption and, when asked,
// a dated retention under the declared ceiling (DAT-14, DAT-17); every other
// operation names the version (ports.ObjectRef). The store refuses to operate
// until Ready confirms the bucket keeps versions and locks them.
//
// It is a provider-block unit over aws-sdk-go-v2. No operation of Store sends
// x-amz-bypass-governance-retention: shortening or purging a GOVERNANCE version
// under a stricter ceiling (DAT-16) is the separate package admin, with its own
// credential.
//
// What this package does not contain: bucket provisioning, the caller's audit
// of the receipts (DAT-25), and multipart or streamed bodies.
package s3
