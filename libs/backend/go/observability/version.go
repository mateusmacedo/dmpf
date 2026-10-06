package observability

// OTelVersion is the OpenTelemetry version every module of the platform pins,
// and version_test.go proves the build graph agrees with it.
const OTelVersion = "1.47.0"

// OTelLogsVersion is the v0 line of the log exporters released with
// OTelVersion; the Logs API and sdk/log left it for the stable line at v1.47.0
// (versions.yaml of the v1.47.0 tag).
const OTelLogsVersion = "0.23.0"

// OTelPrometheusExporterVersion is the v0 line of exporters/prometheus that
// ships with OTelVersion; autoexport requires it (RF-E2).
const OTelPrometheusExporterVersion = "0.69.0"

// SemconvVersion is the semantic conventions version the attributes follow. It
// moves on its own cadence, so it is pinned apart from OTelVersion.
const SemconvVersion = "1.43.0"
