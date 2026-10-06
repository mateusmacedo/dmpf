module github.com/mateusmacedo/dmpf/apps/backend/bff

go 1.26.6

require (
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/contracts v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/grpc v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/http v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/observability v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/testkit v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/transport v1.0.0-rc.0
	github.com/twmb/franz-go v1.21.6
	github.com/twmb/franz-go/pkg/kadm v1.18.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	google.golang.org/grpc v1.85.0-dev.0.20260825072537-93e31b48545e
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/mateusmacedo/dmpf/apps/backend/bookings/contract v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/apps/backend/orders/contract v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/apps/backend/reservations/contract v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/authn v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v1.0.0-rc.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk/log v1.47.0
	go.opentelemetry.io/proto/otlp v1.11.1
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-oidc/v3 v3.21.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v1.0.0-rc.0 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/postgres v1.0.0-rc.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.13.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/bridges/otelslog v0.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.47.0 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
)
