// Package orders is the example aggregate of the DMPF kernel: FND-03 §8.2 and
// §8.3 transcribed to Go. Order has two UPRs, AddItem and Place, each returning
// (domain.Accepted[R], *domain.Rejection) and deciding over a copy so a
// refusal never touches the order (DEC-10, DEC-11). It exists as the executable
// subject for KRN-04 and for the conformance vectors, not as a business module.
package orders
