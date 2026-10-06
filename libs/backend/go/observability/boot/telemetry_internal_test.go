package boot

import "testing"

func TestAnHTTPEndpointOfAnOTLPSignalIsAnExportInThePlain(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"http on the general endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel-collector:4317"}, true},
		{"the scheme in any case", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "HTTP://otel-collector:4317"}, true},
		{"https on the general endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://otel-collector:4317"}, false},
		{"no endpoint declared", map[string]string{}, false},
		{"http on the endpoint of one signal", map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT":      "https://otel-collector:4317",
			"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "http://otel-collector:4317",
		}, true},
		{"https on the endpoint of every signal", map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT":         "http://otel-collector:4317",
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  "https://otel-collector:4317",
			"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "https://otel-collector:4317",
			"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT":    "https://otel-collector:4317",
		}, false},
		{"http without an otlp exporter", map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel-collector:4317",
			"OTEL_TRACES_EXPORTER":        "none",
			"OTEL_METRICS_EXPORTER":       "console",
			"OTEL_LOGS_EXPORTER":          "none",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, variable := range []string{
				"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER", "OTEL_EXPORTER_OTLP_ENDPOINT",
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
			} {
				t.Setenv(variable, tt.env[variable])
			}
			if got := exportsInThePlain(); got != tt.want {
				t.Fatalf("exportsInThePlain() = %t, want %t", got, tt.want)
			}
		})
	}
}
