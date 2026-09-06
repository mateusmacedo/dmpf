package dmpfobservability

// OTelVersion is the OpenTelemetry version every module of the platform pins,
// and version_test.go proves the build graph agrees with it.
const OTelVersion = "1.46.0"

// SemconvVersion is the semantic conventions version the attributes follow. It
// moves on its own cadence, so it is pinned apart from OTelVersion.
const SemconvVersion = "1.43.0"
