package dmpfpostgres

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

// EventMapper turns a domain event into the contract that carries it on the
// wire. It is an interface the provider declares and each bounded context
// realizes: a domain event has no wire form of its own (BLK-03).
type EventMapper interface {
	// Map reports ErrUnmappedEvent for an event with no registered contract, so
	// what nobody can publish rolls back instead of reaching the outbox.
	Map(event dmpfdomain.DomainEvent) (Mapped, error)
}

// Mapped is the contract message plus the envelope type that names it in
// PTB-03 form ("com.company.orders.order-placed.v1").
type Mapped struct {
	Message proto.Message
	Type    string
}

// The two majors come from independent sources — the type is authored by the
// mapper, the package by the generated code — so a mapper pointing a v1 message
// at a v2 type would publish a lie that only this comparison catches (ENV-16).
func checkMajor(mapped Mapped) error {
	if mapped.Message == nil {
		return ErrEmptyMapping
	}
	contract := majorOfDotted(string(mapped.Message.ProtoReflect().Descriptor().ParentFile().Package()))
	declared := majorOfDotted(mapped.Type)
	if contract == "" || contract != declared {
		return fmt.Errorf("%w: contract %q, type %q", envelope.ErrMajorMismatch, contract, declared)
	}
	return nil
}

// "company.orders.event.v1" and "com.company.orders.order-placed.v1" differ in
// everything but this trailing segment, which is what makes them comparable.
func majorOfDotted(dotted string) string {
	dot := strings.LastIndex(dotted, ".")
	if dot < 0 {
		return ""
	}
	return dotted[dot+1:]
}
