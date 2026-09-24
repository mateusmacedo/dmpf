// Package appkit composes the orders application with its concrete
// realizations over Postgres and observes the effects the use case leaves
// behind (KIT-05). App block; its tests carry the integration build tag.
//
// It differs from the appkit of a consuming context: orders only produces, so
// the edge exercised here is the use case and the edge observed is the outbox,
// never the inbox.
package appkit
