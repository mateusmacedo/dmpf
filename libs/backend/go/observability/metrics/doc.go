// Package metrics is the catalogue of the platform's metric series and the
// closed label builder that writes their dimensions.
//
// The builder has one method per permitted key and no generic setter, so a
// forbidden or unbounded label cannot reach a series by accident.
package metrics
