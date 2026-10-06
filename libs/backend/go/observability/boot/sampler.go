package boot

import (
	"os"
	"reflect"
)

const sdkTracePackage = "go.opentelemetry.io/otel/sdk/trace"

// WHY: sdktrace.NewTracerProvider parses OTEL_TRACES_SAMPLER on its own and hands
// what it cannot use to otel.Handle (sdk@v1.47.0/trace/provider.go:533-538,
// sampler_env.go:25-46); the platform already warns once that it ignores it.
func isSamplerVariableError(err error) bool {
	kind := reflect.TypeOf(err)
	return kind != nil && kind.PkgPath() == sdkTracePackage &&
		(kind.Name() == "errUnsupportedSampler" || kind.Name() == "samplerArgParseError")
}

func declaredSampler(fromSignals string) (string, bool) {
	if fromSignals != "" {
		return fromSignals, true
	}
	return os.LookupEnv(EnvTracesSampler)
}
