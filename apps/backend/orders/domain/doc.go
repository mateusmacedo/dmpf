// Package domain is the domain block of the orders bounded context: the Order
// aggregate of FND-03 §8.2 and §8.3 transcribed to Go, with two UPRs (AddItem,
// Place) that decide over a copy so a refusal never touches the order (DEC-10,
// DEC-11). It is the reference context of the kernel topology (ADR-044).
// Its unit and its bounded context are stated in dmpf-units.json (ADR-012).
package domain
