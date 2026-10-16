package provider

import (
	_ "embed"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// Schema is applied by the composition root through postgres.Migrate: running
// DDL here would need the driver's pool, which a context provider does not hold.
//
//go:embed schema.sql
var Schema string

var Grants = []postgres.Grant{postgres.Mutable("orders")}
