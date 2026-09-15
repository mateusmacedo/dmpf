module github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference

go 1.26.6

require (
	github.com/jackc/pgx/v5 v5.10.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-http v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport v0.1.0
	github.com/twmb/franz-go v1.21.6
	github.com/twmb/franz-go/pkg/kadm v1.18.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/sdk v1.46.0
	go.opentelemetry.io/otel/sdk/metric v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	google.golang.org/protobuf v1.36.12
)
