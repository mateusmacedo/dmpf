// Package appkit composes the resource-scheduling application with its
// concrete realizations over Postgres and observes the effects the use case
// leaves behind (KIT-05). App block; its tests carry the integration build tag.
//
// Like the appkit of orders and unlike the one of reservations, this context
// only produces: the edge exercised is the use case and the edge observed is
// the outbox, never the inbox.
package appkit
