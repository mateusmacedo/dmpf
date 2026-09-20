// comment-discipline-ok-file: godoc de package; o que esta realização cobre e o que fica com o chamador é exigência da porta de autenticação, e precisa ficar declarado junto da realização.

// Package authn realizes ports.Authenticator against an OIDC authorization
// server, as the provider unit kernel/provider-authn. Reaching the authority is
// io.network, which CTX-02 places in a provider: the port declares the type,
// the app mounts the instance, and this resolves what the request does not
// carry.
//
// What it covers: discovery of the issuer at start, the JWKS with its cache and
// rotation, verification of signature, issuer, audience and expiry, and the
// reading of subject, tenant and permissions from claims the operator names.
// Every claim is a path, so an issuer that nests them (realm_access.roles) or
// namespaces them (https://app.example.com/tenant_id) needs no code change.
//
// What it does not cover: choosing the identity provider, which FND-07 leaves
// to whoever operates the system, and the shape of a refusal, which belongs to
// the contract each edge publishes. DevAuthenticator resolves identity from the
// credential itself and satisfies no part of IDN-01; the start refuses it
// unless the operator declares it.
package authn
