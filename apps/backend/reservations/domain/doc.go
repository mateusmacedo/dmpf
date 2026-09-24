// Package domain is the domain block of the reservations bounded context: a
// Reservation confirms items for an order through one UPR, Reserve, deciding
// over a copy so a refusal never touches the reservation (DEC-10, DEC-11).
// Its permanent natural key is the OrderID, never a surrogate identifier
// (FND-04 §7.2, GAR-10).
package domain
