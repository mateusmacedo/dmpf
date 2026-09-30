package observability

// OTelVersion is the OpenTelemetry version every module of the platform pins,
// and version_test.go proves the build graph agrees with it.
const OTelVersion = "1.46.0"

// OTelLogsVersion is the release of the log signal that ships with
// OTelVersion: OpenTelemetry Go versions its unstable modules on a v0 line of
// their own (sdk/log v0.22.0 requires otel v1.46.0).
const OTelLogsVersion = "0.22.0"

// SemconvVersion is the semantic conventions version the attributes follow. It
// moves on its own cadence, so it is pinned apart from OTelVersion.
const SemconvVersion = "1.43.0"
