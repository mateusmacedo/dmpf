// comment-discipline-ok-file: godoc de package; a repartição entre repositório e mapeador de exemplo é a mesma matriz de blocos do pacote pai, por exigência da spec do KRN-06.

// Package orderspg is the reference repository and event mapper for the
// kernel/example-orders-postgres unit: the Postgres counterpart of
// application/example/memory, over the same orders.Snapshot and the two
// domain events the aggregate produces (OrderPlaced, ItemAdded).
//
// It realizes ports.Repository[orders.OrderID, orders.Snapshot] with
// optimistic locking in SQL and postgres.EventMapper into the
// company.orders.event.v1 Protobuf contracts. It never opens its own
// transaction: every query runs on the pgx.Tx a postgres.Tx exposes.
package orderspg
