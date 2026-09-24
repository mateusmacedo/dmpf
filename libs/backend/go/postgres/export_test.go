package postgres

import "github.com/jackc/pgx/v5"

// ConnOf lets the tests of Within write through the open transaction without a
// Table: the contract under test is the transaction, not the tenant scope.
func ConnOf(t *Tx) pgx.Tx { return t.conn }
