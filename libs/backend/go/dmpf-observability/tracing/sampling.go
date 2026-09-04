package tracing

// Class is the traffic class of TRC-13. It drives the sampling rate of both the
// trace and the log, so it is declared once here and consumed by otelboot and
// by logging: two copies of the taxonomy would drift.
type Class string

const (
	// ClassError is a failure, always sampled (TRC-14).
	ClassError Class = "error"
	// ClassWrite is traffic that changes state.
	ClassWrite Class = "write"
	// ClassRead is traffic that only reads.
	ClassRead Class = "read"
	// ClassMaintenance is operational traffic, always sampled.
	ClassMaintenance Class = "maintenance"
	// ClassUnclassified is traffic that declared no class, and takes the most
	// restrictive rate.
	ClassUnclassified Class = "unclassified"
)

// Rates maps a class to the fraction of its traffic that is sampled.
type Rates map[Class]float64

// RateMostRestrictive is what an undeclared class resolves to.
const RateMostRestrictive = 0.01

// DefaultRates is the platform baseline of TRC-13.
func DefaultRates() Rates {
	return Rates{
		ClassError:       1.0,
		ClassWrite:       0.10,
		ClassRead:        0.01,
		ClassMaintenance: 1.0,
	}
}

// RateFor is the rate of a class. An absent class takes the most restrictive
// rate rather than passing everything through by omission.
func (r Rates) RateFor(class Class) float64 {
	if rate, declared := r[class]; declared {
		return rate
	}
	return RateMostRestrictive
}
